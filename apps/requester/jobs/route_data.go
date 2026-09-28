package jobs

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/go-co-op/gocron/v2"
	"github.com/samber/lo"
	"github.com/twpayne/go-polyline"

	"github.com/MaroonRides/api/apps/requester/busapi"
	"github.com/MaroonRides/api/apps/requester/repositories"
	"github.com/MaroonRides/api/apps/requester/utils"
	"github.com/MaroonRides/api/internal/db/model"
)

func NewRouteDataJob(client *busapi.Client, repo *repositories.RouteDataRepository) Job {
	return Job{
		Task: func(ctx context.Context) error {
			baseData, err := client.GetBaseData(ctx)
			if err != nil {
				return fmt.Errorf("fetching base data: %w", err)
			}

			rows := routeRows(baseData.Routes)
			routes, err := repo.UpsertRoutes(ctx, rows)
			if err != nil {
				return fmt.Errorf("upserting routes: %w", err)
			}

			err = repo.DeactivateInactiveRoutes(ctx, sourceIDs(rows))
			if err != nil {
				return fmt.Errorf("deactivating inactive routes: %w", err)
			}

			paths, err := client.GetPatternPaths(ctx, sourceIDs(rows))
			if err != nil {
				return fmt.Errorf("fetching pattern paths: %w", err)
			}

			directions, err := repo.UpsertDirections(ctx, directionRows(paths, routes, destinations(baseData.Routes)))
			if err != nil {
				return fmt.Errorf("upserting directions: %w", err)
			}

			stops, err := repo.UpsertStops(ctx, stopRows(paths))
			if err != nil {
				return fmt.Errorf("upserting stops: %w", err)
			}

			if err := repo.UpsertDirectionStops(ctx, directionStopRows(paths, directions, stops)); err != nil {
				return fmt.Errorf("upserting direction stops: %w", err)
			}

			alerts, err := repo.SyncAlerts(ctx, alertRows(baseData.ServiceInterruptions))
			if err != nil {
				return fmt.Errorf("syncing alerts: %w", err)
			}

			if err := repo.SyncAlertDirections(ctx, alertDirectionRows(baseData.Routes, alerts, directions)); err != nil {
				return fmt.Errorf("syncing alert directions: %w", err)
			}

			return nil
		},
		Schedule: gocron.DurationJob(1 * time.Hour),
		Name:     JobRouteData,
	}
}

func routeRows(routes []busapi.MapRoute) []model.Route {
	out := make([]model.Route, 0, len(routes))

	for _, route := range routes {
		if len(route.DirectionList) == 0 {
			slog.Warn("Skipping route with no directions", "route", route.Key)
			continue
		}

		lightColor := route.DirectionList[0].LineColor
		darkColor := utils.DarkModeRouteColor(route.ShortName)
		if darkColor == "" {
			darkColor = lightColor
		}

		out = append(out, model.Route{
			SourceID:   route.Key,
			ShortName:  route.ShortName,
			LongName:   route.Name,
			LightColor: strings.ToLower(lightColor),
			DarkColor:  strings.ToLower(darkColor),
		})
	}

	return out
}

// destination is the headsign and the order a direction is listed in, which
// only the base data carries.
type destination struct {
	text     string
	sequence int
}

func destinations(routes []busapi.MapRoute) map[string]destination {
	out := map[string]destination{}

	for _, route := range routes {
		for sequence, direction := range route.DirectionList {
			out[direction.Direction.Key] = destination{
				text:     direction.Destination,
				sequence: sequence,
			}
		}
	}

	return out
}

func directionRows(paths []busapi.PatternPathsResponse, routes map[string]model.Route, headsigns map[string]destination) []model.Direction {
	var out []model.Direction

	for _, response := range paths {
		route, stored := routes[response.RouteKey]
		if !stored {
			slog.Warn("Pattern path names an unknown route", "route", response.RouteKey)
			continue
		}

		for _, path := range response.PatternPaths {
			headsign := headsigns[path.DirectionKey]

			out = append(out, model.Direction{
				SourceID:    path.DirectionKey,
				RouteID:     route.ID,
				Destination: headsign.text,
				Sequence:    headsign.sequence,
				Path:        encodePath(path.PatternPoints),
			})
		}
	}

	return out
}

func stopRows(paths []busapi.PatternPathsResponse) []model.Stop {
	return lo.FlatMap(paths, func(response busapi.PatternPathsResponse, _ int) []model.Stop {
		return lo.FlatMap(response.PatternPaths, func(path busapi.MapPatternPath, _ int) []model.Stop {
			return lo.Map(stopPoints(path.PatternPoints), func(point busapi.MapPatternPoint, _ int) model.Stop {
				return model.Stop{
					SourceID: point.Stop.StopCode,
					Name:     point.Stop.Name,
					Lat:      point.Latitude,
					Lon:      point.Longitude,
				}
			})
		})
	})
}

func directionStopRows(paths []busapi.PatternPathsResponse, directions map[string]model.Direction, stops map[string]model.Stop) []model.DirectionStop {
	var out []model.DirectionStop

	for _, response := range paths {
		for _, path := range response.PatternPaths {
			direction, stored := directions[path.DirectionKey]
			if !stored {
				slog.Warn("Pattern path names an unknown direction", "direction", path.DirectionKey)
				continue
			}

			for sequence, point := range stopPoints(path.PatternPoints) {
				stop, stored := stops[point.Stop.StopCode]
				if !stored {
					slog.Warn("Pattern point names an unknown stop", "stop", point.Stop.StopCode)
					continue
				}

				out = append(out, model.DirectionStop{
					DirectionID: direction.ID,
					StopID:      stop.ID,
					Sequence:    sequence,
					IsTimepoint: false, // TODO: update this logic
				})
			}
		}
	}

	return out
}

func alertRows(interruptions []busapi.MapServiceInterruption) []model.Alert {
	out := make([]model.Alert, 0, len(interruptions))

	for _, interruption := range interruptions {
		startsAt, err := parseUpstreamTime(interruption.StartDateUtc)
		if err != nil {
			slog.Warn("Skipping alert with unparseable start", "alert", interruption.Key, "error", err)
			continue
		}

		alert := model.Alert{
			SourceID:       interruption.Key,
			Title:          interruption.Name,
			Description:    interruption.Description,
			TimeRangeText:  interruption.TimeRangeString,
			DailyStartTime: interruption.DailyStartTime,
			DailyEndTime:   interruption.DailyEndTime,
			StartsAt:       startsAt,
		}

		// upstream sends an empty end for alerts that run until further notice
		if interruption.EndDateUtc != "" {
			endsAt, err := parseUpstreamTime(interruption.EndDateUtc)
			if err != nil {
				slog.Warn("Skipping alert with unparseable end", "alert", interruption.Key, "error", err)
				continue
			}
			alert.EndsAt = &endsAt
		}

		out = append(out, alert)
	}

	return out
}

func alertDirectionRows(routes []busapi.MapRoute, alerts map[string]model.Alert, directions map[string]model.Direction) []model.AlertDirection {
	var out []model.AlertDirection

	for _, route := range routes {
		for _, listed := range route.DirectionList {
			direction, stored := directions[listed.Direction.Key]
			if !stored {
				continue
			}

			for _, key := range listed.ServiceInterruptionKeys {
				alert, stored := alerts[strconv.Itoa(key)]
				if !stored {
					slog.Warn("Direction names an unknown alert", "direction", listed.Direction.Key, "alert", key)
					continue
				}

				out = append(out, model.AlertDirection{
					AlertID:     alert.ID,
					DirectionID: direction.ID,
				})
			}
		}
	}

	return out
}

func stopPoints(points []busapi.MapPatternPoint) []busapi.MapPatternPoint {
	return lo.Filter(points, func(p busapi.MapPatternPoint, _ int) bool { return p.Stop != nil })
}

func encodePath(points []busapi.MapPatternPoint) string {
	coords := lo.Map(points, func(point busapi.MapPatternPoint, _ int) []float64 {
		return []float64{point.Latitude, point.Longitude}
	})

	return string(polyline.EncodeCoords(coords))
}

func sourceIDs(routes []model.Route) []string {
	return lo.Map(routes, func(r model.Route, _ int) string { return r.SourceID })
}
