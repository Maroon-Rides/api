package jobs

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/go-co-op/gocron/v2"
	"github.com/google/uuid"
	"github.com/samber/lo"
	"golang.org/x/sync/errgroup"

	"github.com/MaroonRides/api/apps/requester/busapi"
	"github.com/MaroonRides/api/apps/requester/repositories"
	"github.com/MaroonRides/api/internal/db/model"
)

const (
	timetableDays        = 10
	timetableConcurrency = 4
	timetableSyncHour    = 2
)

func NewTimetableJob(
	client *busapi.Client,
	routeRepo *repositories.RouteDataRepository,
	repo *repositories.TimetableRepository,
	location *time.Location,
) Job {
	return Job{
		Task: func(ctx context.Context) error {
			baseData, err := client.GetBaseData(ctx)
			if err != nil {
				return fmt.Errorf("fetching base data: %w", err)
			}

			directions, err := routeRepo.GetDirections(ctx)
			if err != nil {
				return fmt.Errorf("fetching directions: %w", err)
			}

			stopDirections, err := routeRepo.GetStopDirectionSourceIDs(ctx)
			if err != nil {
				return fmt.Errorf("fetching stop directions: %w", err)
			}

			stops, err := routeRepo.GetStops(ctx)
			if err != nil {
				return fmt.Errorf("fetching stops: %w", err)
			}

			index := newScheduleDirectionIndex(baseData.Routes, lo.KeyBy(directions, func(d model.Direction) string {
				return d.SourceID
			}))
			stopSourceMap := lo.KeyBy(stops, func(s model.Stop) string { return s.SourceID })
			stopCodes := lo.Keys(stopDirections)

			today := serviceDate(time.Now(), location)

			for day := range timetableDays {
				date := today.AddDate(0, 0, day)
				results := fetchStopSchedules(ctx, client, stopCodes, date)

				fetchedStopIDs := lo.FilterMap(lo.Keys(results), func(sourceID string, _ int) (uuid.UUID, bool) {
					stop, ok := stopSourceMap[sourceID]
					return stop.ID, ok
				})

				rows := stopScheduleRows(results, stopSourceMap, index, date)

				if err := repo.SyncStopSchedules(ctx, date, fetchedStopIDs, rows); err != nil {
					return fmt.Errorf("syncing stop schedules for %s: %w", date.Format(time.DateOnly), err)
				}
			}

			if err := repo.DeleteStopSchedulesBefore(ctx, today); err != nil {
				return fmt.Errorf("deleting past stop schedules: %w", err)
			}

			return nil
		},
		Schedule: gocron.DailyJob(1, gocron.NewAtTimes(gocron.NewAtTime(timetableSyncHour, 0, 0))),
		Name:     JobTimetable,
	}
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

func stopScheduleRows(
	results map[string]busapi.StopSchedulesResponse,
	stops map[string]model.Stop,
	index scheduleDirectionIndex,
	date time.Time,
) []model.StopSchedule {
	var out []model.StopSchedule

	for stopSourceID, response := range results {
		stop, ok := stops[stopSourceID]
		if !ok {
			slog.Warn("Stop schedules name an unknown stop", "stop", stopSourceID)
			continue
		}

		for _, schedule := range response.RouteStopSchedules {
			if len(schedule.StopTimes) == 0 {
				continue
			}

			direction, ok := index.lookup(schedule.RouteNumber, schedule.DirectionName)
			if !ok {
				slog.Debug("Stop schedules name an unknown direction", "route", schedule.RouteNumber, "direction", schedule.DirectionName)
				continue
			}

			for _, stopTime := range schedule.StopTimes {
				scheduledAt, err := parseUpstreamTime(stopTime.ScheduledDepartTimeUtc)
				if err != nil {
					slog.Warn("Skipping unparseable stop time", "stop", stopSourceID, "error", err)
					continue
				}

				out = append(out, model.StopSchedule{
					StopID:      stop.ID,
					DirectionID: direction.ID,
					ScheduledAt: scheduledAt,
					ServiceDate: date,
				})
			}
		}
	}

	return out
}
