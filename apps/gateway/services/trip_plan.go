package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/MaroonRides/api/apps/gateway/dtos"
	"github.com/MaroonRides/api/internal/busapi"
	"github.com/MaroonRides/api/internal/db/model"
)

var ErrTripPlannerUnavailable = errors.New("trip planner unavailable")

var tripRouteOptions = map[dtos.TripPreference]busapi.RouteOption{
	dtos.TripPreferences.Best:           busapi.RouteOptionBest,
	dtos.TripPreferences.FewerTransfers: busapi.RouteOptionFewerTransfers,
	dtos.TripPreferences.LessWalking:    busapi.RouteOptionLessWalking,
}

type TripPlanSource interface {
	GetTripPlan(ctx context.Context, query busapi.TripPlanQuery) (*busapi.TripPlan, error)
	GetRouteStops(ctx context.Context, routeShortNames, codes []string) ([]model.Stop, error)
}

type TripPlanService struct {
	source TripPlanSource
	now    func() time.Time
}

func NewTripPlanService(source TripPlanSource) *TripPlanService {
	return &TripPlanService{source: source, now: time.Now}
}

func (s *TripPlanService) Plan(ctx context.Context, req dtos.TripPlanRequest) (dtos.TripPlanResponse, error) {
	plan, err := s.source.GetTripPlan(ctx, s.query(req))
	if err != nil {
		return dtos.TripPlanResponse{}, fmt.Errorf("%w: %w", ErrTripPlannerUnavailable, err)
	}

	itineraries := newTripItineraries(plan, req.Origin)
	lookup := newTripStopLookup(itineraries)
	stops, err := s.source.GetRouteStops(ctx, lookup.RouteShortNames, lookup.StopCodes)
	if err != nil {
		return dtos.TripPlanResponse{}, err
	}
	resolveTripStops(itineraries, stops)

	return dtos.TripPlanResponse{Itineraries: itineraries}, nil
}

func (s *TripPlanService) query(req dtos.TripPlanRequest) busapi.TripPlanQuery {
	at := s.now()
	if req.Time != nil {
		at = *req.Time
	}

	query := busapi.TripPlanQuery{
		Origin:      tripEndpoint(req.Origin),
		Destination: tripEndpoint(req.Destination),
		RouteOption: tripRouteOptions[req.Preference],
	}
	if req.TimeKind == dtos.TripTimeKinds.ArriveBy {
		query.ArriveBy = &at
	} else {
		query.DepartAt = &at
	}
	return query
}

func tripEndpoint(e dtos.TripEndpoint) busapi.Endpoint {
	return busapi.Endpoint{Title: e.Name, Latitude: &e.Lat, Longitude: &e.Lon}
}
