package services

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/samber/lo"
	"golang.org/x/sync/errgroup"

	"github.com/MaroonRides/api/apps/requester/repositories"
	"github.com/MaroonRides/api/apps/requester/repositories/busapi"
	"github.com/MaroonRides/api/internal/db/model"
)

const (
	timetableDays        = 10
	timetableConcurrency = 4
)

type TimetableService struct {
	api       *busapi.Client
	routeRepo *repositories.RouteDataRepository
	repo      *repositories.TimetableRepository
	location  *time.Location
}

func NewTimetableService(
	api *busapi.Client,
	routeRepo *repositories.RouteDataRepository,
	repo *repositories.TimetableRepository,
	location *time.Location,
) *TimetableService {
	return &TimetableService{api: api, routeRepo: routeRepo, repo: repo, location: location}
}

func (s *TimetableService) Sync(ctx context.Context) error {
	baseData, err := s.api.GetBaseData(ctx)
	if err != nil {
		return fmt.Errorf("fetching base data: %w", err)
	}

	directions, err := s.routeRepo.GetDirections(ctx)
	if err != nil {
		return fmt.Errorf("fetching directions: %w", err)
	}

	stopDirections, err := s.routeRepo.GetStopDirectionSourceIDs(ctx)
	if err != nil {
		return fmt.Errorf("fetching stop directions: %w", err)
	}

	stops, err := s.routeRepo.GetStops(ctx)
	if err != nil {
		return fmt.Errorf("fetching stops: %w", err)
	}

	index := newScheduleDirectionIndex(baseData.Routes, keyDirectionsBySource(directions))
	stopsByKey := lo.KeyBy(stops, func(s model.Stop) string { return repositories.StopKey(s.DirectionID, s.SourceID) })
	stopCodes := lo.Keys(stopDirections)

	today := serviceDate(time.Now(), s.location)

	for day := range timetableDays {
		date := today.AddDate(0, 0, day)
		results := fetchStopSchedules(ctx, s.api, stopCodes, date)

		fetchedStopIDs := lo.FilterMap(stops, func(stop model.Stop, _ int) (uuid.UUID, bool) {
			_, fetched := results[stop.SourceID]
			return stop.ID, fetched
		})

		rows := timetableRows(results, stopsByKey, index, date)

		if err := s.repo.SyncTimetables(ctx, date, fetchedStopIDs, rows); err != nil {
			return fmt.Errorf("syncing timetables for %s: %w", date.Format(time.DateOnly), err)
		}
	}

	if err := s.repo.DeleteTimetablesBefore(ctx, today); err != nil {
		return fmt.Errorf("deleting past timetables: %w", err)
	}

	return nil
}

func serviceDate(now time.Time, location *time.Location) time.Time {
	year, month, day := now.In(location).Date()
	return time.Date(year, month, day, 0, 0, 0, 0, location)
}

// fetchStopSchedules leaves out stops whose request failed, so their stored
// schedules survive until the next run.
func fetchStopSchedules(ctx context.Context, client *busapi.Client, stopCodes []string, date time.Time) map[string]busapi.StopSchedulesResponse {
	var mu sync.Mutex
	results := make(map[string]busapi.StopSchedulesResponse, len(stopCodes))

	var group errgroup.Group
	group.SetLimit(timetableConcurrency)

	for _, stopCode := range stopCodes {
		group.Go(func() error {
			schedules, err := client.GetStopSchedules(ctx, stopCode, date)
			if err != nil {
				slog.Error("Failed to fetch stop schedules", "stop", stopCode, "date", date.Format(time.DateOnly), "error", err)
				return nil
			}

			mu.Lock()
			results[stopCode] = *schedules
			mu.Unlock()
			return nil
		})
	}

	_ = group.Wait()
	return results
}

// scheduleDirectionIndex finds a direction by the route number and direction
// name that stop schedules carry in place of keys.
type scheduleDirectionIndex map[string]map[string]model.Direction

func newScheduleDirectionIndex(routes []busapi.MapRoute, directions map[string]model.Direction) scheduleDirectionIndex {
	index := scheduleDirectionIndex{}

	for _, route := range routes {
		for _, listed := range route.DirectionList {
			direction, stored := directions[listed.Direction.Key]
			if !stored {
				continue
			}

			if index[route.ShortName] == nil {
				index[route.ShortName] = map[string]model.Direction{}
			}
			index[route.ShortName][listed.Direction.Name] = direction
		}
	}

	return index
}

func (index scheduleDirectionIndex) lookup(routeNumber, directionName string) (model.Direction, bool) {
	direction, ok := index[routeNumber][directionName]
	return direction, ok
}

func timetableRows(
	results map[string]busapi.StopSchedulesResponse,
	stops map[string]model.Stop,
	index scheduleDirectionIndex,
	date time.Time,
) []model.Timetable {
	departures := map[uuid.UUID][]time.Time{}

	for stopSourceID, response := range results {
		for _, schedule := range response.RouteStopSchedules {
			if len(schedule.StopTimes) == 0 {
				continue
			}

			direction, ok := index.lookup(schedule.RouteNumber, schedule.DirectionName)
			if !ok {
				slog.Debug("Stop schedules name an unknown direction", "route", schedule.RouteNumber, "direction", schedule.DirectionName)
				continue
			}

			stop, ok := stops[repositories.StopKey(direction.ID, stopSourceID)]
			if !ok {
				slog.Debug("Stop schedules name a stop the direction does not serve", "stop", stopSourceID, "direction", direction.SourceID)
				continue
			}

			for _, stopTime := range schedule.StopTimes {
				scheduledAt, err := parseUpstreamTime(stopTime.ScheduledDepartTimeUtc)
				if err != nil {
					slog.Warn("Skipping unparseable stop time", "stop", stopSourceID, "error", err)
					continue
				}

				departures[stop.ID] = append(departures[stop.ID], scheduledAt)
			}
		}
	}

	return lo.MapToSlice(departures, func(stopID uuid.UUID, times []time.Time) model.Timetable {
		slices.SortFunc(times, time.Time.Compare)
		return model.Timetable{
			StopID:      stopID,
			ServiceDate: date,
			Departures:  slices.CompactFunc(times, time.Time.Equal),
		}
	})
}
