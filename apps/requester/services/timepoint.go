package services

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/MaroonRides/api/apps/requester/repositories"
	"github.com/MaroonRides/api/apps/requester/repositories/gtfs"
)

type TimepointService struct {
	feed *gtfs.Client
	repo *repositories.RouteDataRepository
}

func NewTimepointService(feed *gtfs.Client, repo *repositories.RouteDataRepository) *TimepointService {
	return &TimepointService{feed: feed, repo: repo}
}

func (s *TimepointService) Sync(ctx context.Context) error {
	feed, err := s.feed.GetFeed(ctx)
	if err != nil {
		return fmt.Errorf("fetching gtfs feed: %w", err)
	}

	refs, err := s.repo.GetStopRefs(ctx)
	if err != nil {
		return fmt.Errorf("fetching stops: %w", err)
	}

	changes := timepointChanges(refs, feedTimepoints(feed))

	if err := s.repo.SetTimepoints(ctx, changes.marked, true); err != nil {
		return fmt.Errorf("marking timepoints: %w", err)
	}

	if err := s.repo.SetTimepoints(ctx, changes.cleared, false); err != nil {
		return fmt.Errorf("clearing timepoints: %w", err)
	}

	return nil
}

type routeStop struct {
	routeShortName string
	stopCode       string
}

// feedTimepoints holds every route stop the feed schedules, true where any trip times it exactly.
func feedTimepoints(feed *gtfs.Feed) map[routeStop]bool {
	routeShortNames := make(map[string]string, len(feed.Routes))
	for _, route := range feed.Routes {
		routeShortNames[route.RouteID] = route.ShortName
	}

	stopCodes := make(map[string]string, len(feed.Stops))
	for _, stop := range feed.Stops {
		stopCodes[stop.StopID] = stop.StopCode
	}

	tripRoutes := make(map[string]string, len(feed.Trips))
	for _, trip := range feed.Trips {
		tripRoutes[trip.TripID] = routeShortNames[trip.RouteID]
	}

	timepoints := map[routeStop]bool{}
	for _, stopTime := range feed.StopTimes {
		key := routeStop{routeShortName: tripRoutes[stopTime.TripID], stopCode: stopCodes[stopTime.StopID]}
		timepoints[key] = timepoints[key] || stopTime.Timepoint
	}

	return timepoints
}

type timepointChangeSet struct {
	marked  []uuid.UUID
	cleared []uuid.UUID
}

// timepointChanges leaves stops the feed does not schedule as they are.
func timepointChanges(refs []repositories.StopRef, timepoints map[routeStop]bool) timepointChangeSet {
	var changes timepointChangeSet

	for _, ref := range refs {
		isTimepoint, scheduled := timepoints[routeStop{routeShortName: ref.RouteShortName, stopCode: ref.StopSourceID}]
		if !scheduled || isTimepoint == ref.IsTimepoint {
			continue
		}

		if isTimepoint {
			changes.marked = append(changes.marked, ref.ID)
		} else {
			changes.cleared = append(changes.cleared, ref.ID)
		}
	}

	return changes
}
