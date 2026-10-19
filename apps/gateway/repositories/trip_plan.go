package repositories

import (
	"context"

	"github.com/uptrace/bun"

	"github.com/MaroonRides/api/internal/busapi"
	"github.com/MaroonRides/api/internal/db/model"
)

type TripPlanRepository struct {
	db     *bun.DB
	busapi *busapi.Client
}

func NewTripPlanRepository(db *bun.DB, client *busapi.Client) *TripPlanRepository {
	return &TripPlanRepository{db: db, busapi: client}
}

func (r *TripPlanRepository) GetTripPlan(ctx context.Context, query busapi.TripPlanQuery) (*busapi.TripPlan, error) {
	return r.busapi.GetTripPlan(ctx, query)
}

// GetRouteStops returns the stops with one of codes on a route with one of routeShortNames,
// with their direction and route loaded.
func (r *TripPlanRepository) GetRouteStops(ctx context.Context, routeShortNames, codes []string) ([]model.Stop, error) {
	stops := []model.Stop{}
	if len(routeShortNames) == 0 || len(codes) == 0 {
		return stops, nil
	}
	err := r.db.NewSelect().
		Model(&stops).
		Relation("Direction").
		Relation("Direction.Route").
		Where("? IN (?)", bun.Ident("direction__route.shortName"), bun.In(routeShortNames)).
		Where("? IN (?)", bun.Ident("stop.sourceId"), bun.In(codes)).
		Scan(ctx)
	return stops, err
}
