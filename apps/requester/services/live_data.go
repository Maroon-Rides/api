package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/MaroonRides/api/apps/requester/repositories"
	"github.com/MaroonRides/api/apps/requester/repositories/busapi"
	"github.com/MaroonRides/api/apps/requester/utils"
	"github.com/MaroonRides/api/internal/db/model"

	"github.com/google/uuid"
	"github.com/samber/lo"
	"golang.org/x/sync/errgroup"
)

const nextDeparturesConcurrency = 4

type LiveDataService struct {
	api  *busapi.Client
	repo *repositories.RouteDataRepository
}

func NewLiveDataService(api *busapi.Client, repo *repositories.RouteDataRepository) *LiveDataService {
	return &LiveDataService{api: api, repo: repo}
}

func (s *LiveDataService) SubscriptionAdded(ctx context.Context, routeID uuid.UUID) error {
	// Postgres notifies when a new subscription to a route is added so we can immediately populate data

	route, err := s.repo.GetRoute(ctx, routeID)
	if err != nil {
		return fmt.Errorf("fetching route: %w", err)
	}

	err = s.SyncRouteVehicles(ctx, route)
	if err != nil {
		return fmt.Errorf("syncing route vehicles: %w", err)
	}

	err = s.SyncRouteDepartures(ctx, routeID)
	if err != nil {
		return fmt.Errorf("syncing route departures: %w", err)
	}

	if err := s.repo.MarkLiveDataAvailable(ctx, routeID); err != nil {
		return fmt.Errorf("marking live data available: %w", err)
	}

	return nil
}

func (s *LiveDataService) SyncActiveVehicles(ctx context.Context) error {
	routes, err := s.repo.GetActiveRoutes(ctx)
	if err != nil {
		return fmt.Errorf("fetching active routes: %w", err)
	}

	return s.SyncVehicles(ctx, routes)
}

func (s *LiveDataService) SyncRouteVehicles(ctx context.Context, route model.Route) error {
	return s.SyncVehicles(ctx, []model.Route{route})
}

func (s *LiveDataService) SyncVehicles(ctx context.Context, routes []model.Route) error {
	if len(routes) == 0 {
		return nil
	}

	directions, err := s.repo.GetDirections(ctx)
	if err != nil {
		return fmt.Errorf("fetching directions: %w", err)
	}

	vehicles, err := s.api.GetVehicles(ctx, sourceIDs(routes))
	if err != nil {
		return fmt.Errorf("fetching vehicles: %w", err)
	}

	rows := vehicleRows(vehicles, keyRoutesBySource(routes), keyDirectionsBySource(directions))

	if err := s.repo.SyncVehicles(ctx, routeIDs(routes), rows); err != nil {
		return fmt.Errorf("syncing vehicles: %w", err)
	}

	return nil
}

func (s *LiveDataService) SyncSubscribedDepartures(ctx context.Context) error {
	routeIDs, err := s.repo.GetSubscribedRouteIDs(ctx, model.LiveDataSubscriptionStaleAfter)
	if err != nil {
		return fmt.Errorf("fetching subscribed routes: %w", err)
	}

	return s.syncRoutesDepartures(ctx, routeIDs)
}

// ReapStaleSubscriptions drops rows left by gateways that stopped refreshing them,
// so a route with no live gateway goes back to unavailable.
func (s *LiveDataService) ReapStaleSubscriptions(ctx context.Context) error {
	reaped, err := s.repo.DeleteStaleSubscriptions(ctx, model.LiveDataSubscriptionStaleAfter)
	if err != nil {
		return fmt.Errorf("deleting stale subscriptions: %w", err)
	}
	if reaped > 0 {
		slog.Info("Reaped stale live data subscriptions", "count", reaped)
	}
	return nil
}

func (s *LiveDataService) SyncRouteDepartures(ctx context.Context, routeID uuid.UUID) error {
	return s.syncRoutesDepartures(ctx, []uuid.UUID{routeID})
}

func (s *LiveDataService) syncRoutesDepartures(ctx context.Context, routeIDs []uuid.UUID) error {
	targets, err := s.repo.GetDepartureTargets(ctx, routeIDs)
	if err != nil {
		return fmt.Errorf("fetching departure targets: %w", err)
	}

	return s.SyncDepartures(ctx, targets)
}

func (s *LiveDataService) SyncDepartures(ctx context.Context, targets []repositories.DepartureTarget) error {
	if len(targets) == 0 {
		return nil
	}

	targetsByStop := lo.GroupBy(targets, func(t repositories.DepartureTarget) string {
		return t.StopSourceID
	})

	results := s.fetchNextDepartureTimes(ctx, targetsByStop)

	err := s.repo.UpdateStopAmenities(ctx, lo.MapValues(results, func(r busapi.NextDepartureTimesResponse, _ string) []string {
		return utils.AmenityNames(r.Amenities)
	}))
	if err != nil {
		return fmt.Errorf("updating stop amenities: %w", err)
	}

	fetchedStopIDs := lo.FlatMap(lo.Keys(results), func(stopSourceID string, _ int) []uuid.UUID {
		return lo.Map(targetsByStop[stopSourceID], func(t repositories.DepartureTarget, _ int) uuid.UUID { return t.StopID })
	})

	departures := departureRows(results, lo.KeyBy(targets, keyDepartureTarget))

	if err := s.repo.SyncDepartures(ctx, fetchedStopIDs, departures); err != nil {
		return fmt.Errorf("syncing departures: %w", err)
	}

	return nil
}

func (s *LiveDataService) fetchNextDepartureTimes(ctx context.Context, targetsByStop map[string][]repositories.DepartureTarget) map[string]busapi.NextDepartureTimesResponse {
	var mu sync.Mutex
	results := make(map[string]busapi.NextDepartureTimesResponse)

	var group errgroup.Group
	group.SetLimit(nextDeparturesConcurrency)

	for stopID, targets := range targetsByStop {
		group.Go(func() error {
			routeDirectionPairs := lo.Map(targets, func(t repositories.DepartureTarget, _ int) busapi.RouteDirectionPair {
				return busapi.RouteDirectionPair{
					RouteKey:     t.RouteSourceID,
					DirectionKey: t.DirectionSourceID,
				}
			})

			estimates, err := s.api.GetNextDepartureTimes(ctx, routeDirectionPairs, stopID)
			if err != nil {
				slog.Error("Failed to fetch next departure times", "stop", stopID, "error", err)
				return nil
			}

			mu.Lock()
			results[stopID] = *estimates
			mu.Unlock()
			return nil
		})
	}

	_ = group.Wait()

	return results
}

func vehicleRows(vehicles []busapi.VehicleResponse, routes map[string]model.Route, directions map[string]model.Direction) []model.Vehicle {
	return lo.FlatMap(vehicles, func(route busapi.VehicleResponse, _ int) []model.Vehicle {
		return lo.FlatMap(route.VehiclesByDirections, func(direction busapi.VehicleByDirection, _ int) []model.Vehicle {
			return lo.FilterMap(direction.Vehicles, func(v busapi.Vehicle, _ int) (model.Vehicle, bool) {
				dbRoute, routeOk := routes[route.RouteKey]
				dbDirection, directionOk := directions[direction.DirectionKey]
				if !routeOk || !directionOk {
					slog.Warn("Skipping vehicle with unknown route or direction", "route", route.RouteKey, "direction", direction.DirectionKey)
					return model.Vehicle{}, false
				}

				return model.Vehicle{
					Name:        v.Name,
					SourceID:    v.Key,
					RouteID:     dbRoute.ID,
					DirectionID: dbDirection.ID,
					Lat:         v.Location.Latitude,
					Lon:         v.Location.Longitude,
					Heading:     v.Location.Heading,
					Speed:       v.Location.Speed,
					Capacity:    v.PassengerCapacity,
					Passengers:  v.PassengersOnboard,
					SeenAt:      time.Now(),

					Amenities: utils.AmenityNames(v.Amenities),
				}, true
			})
		})
	})
}

type departureTargetKey struct {
	stop, route, direction string
}

func keyDepartureTarget(t repositories.DepartureTarget) departureTargetKey {
	return departureTargetKey{stop: t.StopSourceID, route: t.RouteSourceID, direction: t.DirectionSourceID}
}

func departureRows(
	results map[string]busapi.NextDepartureTimesResponse,
	targets map[departureTargetKey]repositories.DepartureTarget,
) []model.Departure {
	var out []model.Departure

	for stopSourceID, response := range results {
		for _, times := range response.RouteDirectionTimes {
			target, ok := targets[departureTargetKey{stop: stopSourceID, route: times.RouteKey, direction: times.DirectionKey}]
			if !ok {
				slog.Warn("Departure times name an unrequested route or direction", "stop", stopSourceID, "route", times.RouteKey, "direction", times.DirectionKey)
				continue
			}

			for _, depart := range times.NextDeparts {
				departure, err := departureRow(depart)
				if err != nil {
					slog.Warn("Skipping unparseable departure", "stop", stopSourceID, "error", err)
					continue
				}

				departure.RouteID = target.RouteID
				departure.StopID = target.StopID
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

func keyRoutesBySource(routes []model.Route) map[string]model.Route {
	return lo.KeyBy(routes, func(r model.Route) string { return r.SourceID })
}

func keyDirectionsBySource(directions []model.Direction) map[string]model.Direction {
	return lo.KeyBy(directions, func(d model.Direction) string { return d.SourceID })
}

func routeIDs(routes []model.Route) []uuid.UUID {
	return lo.Map(routes, func(r model.Route, _ int) uuid.UUID { return r.ID })
}
