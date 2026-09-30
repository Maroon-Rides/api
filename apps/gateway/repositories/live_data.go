package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/uptrace/bun"

	"github.com/MaroonRides/api/internal/db/model"
)

type LiveDataRepository struct {
	db *bun.DB
}

func NewLiveDataRepository(db *bun.DB) *LiveDataRepository {
	return &LiveDataRepository{db: db}
}

// SyncLiveDataSubscriptions makes routeIDs the only subscriptions held by clientID and refreshes their lastUpdatedAt.
func (r *LiveDataRepository) SyncLiveDataSubscriptions(ctx context.Context, clientID uuid.UUID, routeIDs []uuid.UUID) error {
	return r.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		stale := tx.NewDelete().
			Model((*model.LiveDataSubscription)(nil)).
			Where("? = ?", bun.Ident("clientId"), clientID)
		if len(routeIDs) > 0 {
			stale = stale.Where("? NOT IN (?)", bun.Ident("routeId"), bun.In(routeIDs))
		}
		if _, err := stale.Exec(ctx); err != nil {
			return err
		}

		if len(routeIDs) == 0 {
			return nil
		}

		subscriptions := lo.Map(routeIDs, func(routeID uuid.UUID, _ int) model.LiveDataSubscription {
			return model.LiveDataSubscription{RouteID: routeID, ClientID: clientID}
		})
		_, err := tx.NewInsert().
			Model(&subscriptions).
			Column("routeId", "clientId").
			On("CONFLICT (?, ?) DO UPDATE", bun.Ident("routeId"), bun.Ident("clientId")).
			Set("? = NOW()", bun.Ident("lastUpdatedAt")).
			Exec(ctx)
		return err
	})
}

func (r *LiveDataRepository) IsLiveDataAvailable(ctx context.Context, routeID uuid.UUID) (bool, error) {
	var available bool
	err := r.db.NewSelect().
		Model((*model.Route)(nil)).
		Column("liveDataAvailable").
		Where("? = ?", bun.Ident("id"), routeID).
		Scan(ctx, &available)
	return available, err
}

func (r *LiveDataRepository) GetRouteVehicles(ctx context.Context, routeID uuid.UUID) ([]model.Vehicle, error) {
	vehicles := []model.Vehicle{}
	err := r.db.NewSelect().
		Model(&vehicles).
		Where("? = ?", bun.Ident("routeId"), routeID).
		OrderExpr("? ASC", bun.Ident("name")).
		Scan(ctx)
	return vehicles, err
}

func (r *LiveDataRepository) GetRouteDepartures(ctx context.Context, routeID uuid.UUID) ([]model.Departure, error) {
	departures := []model.Departure{}
	err := r.db.NewSelect().
		Model(&departures).
		Where("? = ?", bun.Ident("routeId"), routeID).
		OrderExpr("? ASC, ? ASC", bun.Ident("stopId"), bun.Ident("scheduledAt")).
		Scan(ctx)
	return departures, err
}
