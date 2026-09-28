package jobs

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/MaroonRides/api/apps/requester/busapi"
	"github.com/MaroonRides/api/apps/requester/repositories"
	"github.com/MaroonRides/api/apps/requester/utils"
	"github.com/MaroonRides/api/internal/db/model"

	"github.com/go-co-op/gocron/v2"
	"github.com/google/uuid"
	"github.com/samber/lo"
	"golang.org/x/sync/errgroup"
)

const nextDeparturesConcurrency = 4

func NewLiveDataJob(api *busapi.Client, repo *repositories.RouteDataRepository, scheduler *Scheduler) Job {
	return Job{
		Task: func(ctx context.Context) error {
			routes, err := repo.GetActiveRoutes(ctx)
			if err != nil {
				slog.Error("Failed to fetch active routes", "error", err)
				return nil
			}

			apiActiveRoutes, err := api.GetActiveRoutes(ctx)
			if err != nil {
				slog.Error("Failed to fetch active routes from api", "error", err)
				return nil
			}

			if !activeRoutesSynced(apiActiveRoutes, routes) {
				slog.Warn("Active routes have been updated, triggering refresh of data.")

				err := scheduler.RunNow(JobRouteData)
				if err != nil {
					slog.Error("Failed to trigger route data refresh", "error", err)
				}

				return nil
			}

			directions, err := repo.GetDirections(ctx)
			if err != nil {
				slog.Error("Failed to fetch directions", "error", err)
				return nil
			}

			routePaths, err := api.GetPatternPaths(ctx, sourceIDs(routes))

			if err != nil {
				slog.Error("Failed to fetch vehicles", "error", err)
				return nil
			}

			// check for empty pattern paths indicating a route data change
			for _, route := range routePaths {
				if len(route.PatternPaths) == 0 && len(route.VehiclesByDirections) == 0 {
					slog.Warn("Route data appears to have changed, triggering refresh of data.")

					err := scheduler.RunNow(JobRouteData)
					if err != nil {
						slog.Error("Failed to trigger route data refresh", "error", err)
					}

					return nil
				}
			}

			routeSourceMap := lo.KeyBy(routes, func(r model.Route) string {
				return r.SourceID
			})
			directionSourceMap := lo.KeyBy(directions, func(d model.Direction) string {
				return d.SourceID
			})

			vehicleRows := lo.FlatMap(routePaths, func(route busapi.PatternPathsResponse, _ int) []model.Vehicle {
				return lo.FlatMap(route.VehiclesByDirections, func(direction busapi.VehicleByDirection, _ int) []model.Vehicle {
					return lo.Map(direction.Vehicles, func(v busapi.Vehicle, _ int) model.Vehicle {
						amenities := utils.AmenityNames(v.Amenities)

						routeID := routeSourceMap[route.RouteKey].ID
						directionID := directionSourceMap[direction.DirectionKey].ID

						if routeID == uuid.Nil || directionID == uuid.Nil {
							slog.Warn("Skipping vehicle with missing route or direction", "routeKey", routeID, "directionKey", directionID)
							return model.Vehicle{}
						}

						return model.Vehicle{
							Name:        v.Name,
							SourceID:    v.Key,
							RouteID:     routeID,
							DirectionID: directionID,
							Lat:         v.Location.Latitude,
							Lon:         v.Location.Longitude,
							Heading:     v.Location.Heading,
							Speed:       v.Location.Speed,
							Capacity:    v.PassengerCapacity,
							Passengers:  v.PassengersOnboard,
							SeenAt:      time.Now(),

							Amenities: amenities,
						}
					})
				})
			})

			// filter out vehicles with missing IDs (skipped)
			vehicleRows = lo.Filter(vehicleRows, func(v model.Vehicle, _ int) bool {
				return v.ID != uuid.Nil
			})

			err = repo.SyncVehicles(ctx, vehicleRows)

			if err != nil {
				slog.Error("Failed to sync vehicles", "error", err)
				return nil
			}

			stopDirections, err := repo.GetStopDirectionSourceIDs(ctx)
			if err != nil {
				slog.Error("Failed to fetch stop directions", "error", err)
				return nil
			}

			var mu sync.Mutex
			var results = make(map[string]busapi.NextDepartureTimesResponse)

			var group errgroup.Group
			group.SetLimit(nextDeparturesConcurrency)

			for stopID, directions := range stopDirections {
				group.Go(func() error {
					routeDirectionPairs := lo.Map(directions, func(ds repositories.DirectionSourceIDs, _ int) busapi.RouteDirectionPair {
						return busapi.RouteDirectionPair{
							RouteKey:     ds.RouteSourceID,
							DirectionKey: ds.DirectionSourceID,
						}
					})

					estimates, err := api.GetNextDepartureTimes(ctx, routeDirectionPairs, stopID)

					if err != nil {
						slog.Error("Failed to fetch next departure times", "error", err)
						return nil
					}

					mu.Lock()
					results[stopID] = *estimates
					mu.Unlock()
					return nil
				})
			}

			_ = group.Wait()

			stops, err := repo.GetStops(ctx)
			if err != nil {
				slog.Error("Failed to fetch stops", "error", err)
				return nil
			}

			stopSourceMap := lo.KeyBy(stops, func(s model.Stop) string {
				return s.SourceID
			})

			err = repo.UpdateStopAmenities(ctx, lo.MapValues(results, func(r busapi.NextDepartureTimesResponse, _ string) []string {
				return utils.AmenityNames(r.Amenities)
			}))
			if err != nil {
				slog.Error("Failed to update stop amenities", "error", err)
				return nil
			}

			fetchedStopIDs := lo.FilterMap(lo.Keys(results), func(sourceID string, _ int) (uuid.UUID, bool) {
				stop, ok := stopSourceMap[sourceID]
				return stop.ID, ok
			})

			departures := departureRows(results, stopSourceMap, routeSourceMap, directionSourceMap)

			err = repo.SyncDepartures(ctx, fetchedStopIDs, departures)
			if err != nil {
				slog.Error("Failed to sync departures", "error", err)
				return nil
			}

			return nil
		},
		Schedule: gocron.DurationJob(10 * time.Second),
		Name:     JobLiveData,
	}
}

func departureRows(
	results map[string]busapi.NextDepartureTimesResponse,
	stops map[string]model.Stop,
	routes map[string]model.Route,
	directions map[string]model.Direction,
) []model.Departure {
	var out []model.Departure

	for stopSourceID, response := range results {
		stop, ok := stops[stopSourceID]
		if !ok {
			slog.Warn("Departure times name an unknown stop", "stop", stopSourceID)
			continue
		}

		for _, times := range response.RouteDirectionTimes {
			route, routeOk := routes[times.RouteKey]
			direction, directionOk := directions[times.DirectionKey]
			if !routeOk || !directionOk {
				slog.Warn("Departure times name an unknown route or direction", "route", times.RouteKey, "direction", times.DirectionKey)
				continue
			}

			for _, depart := range times.NextDeparts {
				departure, err := departureRow(depart)
				if err != nil {
					slog.Warn("Skipping unparseable departure", "stop", stopSourceID, "error", err)
					continue
				}

				departure.RouteID = route.ID
				departure.StopID = stop.ID
				departure.DirectionID = direction.ID
				out = append(out, departure)
			}
		}
	}

	return out
}

func departureRow(depart busapi.DepartureTime) (model.Departure, error) {
	if depart.ScheduledDepartTimeUtc == nil {
		return model.Departure{}, errors.New("missing scheduled time")
	}

	scheduledAt, err := parseUpstreamTime(*depart.ScheduledDepartTimeUtc)
	if err != nil {
		return model.Departure{}, err
	}

	departure := model.Departure{ScheduledAt: scheduledAt}

	if depart.EstimatedDepartTimeUtc != nil {
		estimatedAt, err := parseUpstreamTime(*depart.EstimatedDepartTimeUtc)
		if err != nil {
			return model.Departure{}, err
		}
		departure.EstimatedAt = &estimatedAt
	}

	return departure, nil
}

// Upstream names its fields "Utc" but may omit the zone suffix.
const upstreamZonelessTimeLayout = "2006-01-02T15:04:05.999999999"

func parseUpstreamTime(value string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return t, nil
	}

	return time.ParseInLocation(upstreamZonelessTimeLayout, value, time.UTC)
}

func activeRoutesSynced(apiRoutes []string, dbRoutes []model.Route) bool {
	dbRouteCodes := lo.Map(dbRoutes, func(r model.Route, _ int) string {
		return r.ShortName
	})

	return len(lo.Without(apiRoutes, dbRouteCodes...)) == 0
}
