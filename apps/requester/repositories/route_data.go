package repositories

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/MaroonRides/api/internal/db/model"
	"github.com/samber/lo"
	"github.com/uptrace/bun"
)

type RouteDataRepository struct {
	db *bun.DB
}

func NewRouteDataRepository(db *bun.DB) *RouteDataRepository {
	return &RouteDataRepository{db: db}
}

func (r *RouteDataRepository) GetActiveRoutes(ctx context.Context) ([]model.Route, error) {
	var routes []model.Route
	err := r.db.NewSelect().Model(&[]model.Route{}).Where("active = TRUE").Scan(ctx, &routes)

	return routes, err
}

func (r *RouteDataRepository) GetRoute(ctx context.Context, routeID uuid.UUID) (model.Route, error) {
	var route model.Route
	err := r.db.NewSelect().Model(&route).Where("? = ?", bun.Ident("id"), routeID).Scan(ctx)

	return route, err
}

func (r *RouteDataRepository) GetDirections(ctx context.Context) ([]model.Direction, error) {
	var directions []model.Direction
	err := r.db.NewSelect().Model(&[]model.Direction{}).Scan(ctx, &directions)

	return directions, err
}

type DirectionSourceIDs struct {
	DirectionSourceID string `json:"directionSourceId"`
	RouteSourceID     string `json:"routeSourceId"`
}

type stopDirectionSourceIDs struct {
	StopSourceID string               `bun:"stopSourceId"`
	Directions   []DirectionSourceIDs `bun:"directions,type:json"`
}

func (r *RouteDataRepository) GetStopDirectionSourceIDs(ctx context.Context) (map[string][]DirectionSourceIDs, error) {
	var rows []stopDirectionSourceIDs
	err := r.db.NewSelect().
		TableExpr(`"direction_stop" AS ds`).
		ColumnExpr(`s."sourceId" AS "stopSourceId"`).
		ColumnExpr(`json_agg(json_build_object('directionSourceId', d."sourceId", 'routeSourceId', r."sourceId")) AS "directions"`).
		Join(`JOIN "stop" AS s ON s."id" = ds."stopId"`).
		Join(`JOIN "direction" AS d ON d."id" = ds."directionId"`).
		Join(`JOIN "route" AS r ON r."id" = d."routeId"`).
		Where("r.active = TRUE").
		GroupExpr(`s."sourceId"`).
		Scan(ctx, &rows)
	if err != nil {
		return nil, err
	}

	return lo.SliceToMap(rows, func(row stopDirectionSourceIDs) (string, []DirectionSourceIDs) {
		return row.StopSourceID, row.Directions
	}), nil
}

// DepartureTarget is one direction of a route serving a stop, with the ids on both sides of the bus API.
type DepartureTarget struct {
	StopID            uuid.UUID `bun:"stopId"`
	StopSourceID      string    `bun:"stopSourceId"`
	RouteID           uuid.UUID `bun:"routeId"`
	RouteSourceID     string    `bun:"routeSourceId"`
	DirectionID       uuid.UUID `bun:"directionId"`
	DirectionSourceID string    `bun:"directionSourceId"`
}

func (r *RouteDataRepository) GetDepartureTargets(ctx context.Context, routeIDs []uuid.UUID) ([]DepartureTarget, error) {
	if len(routeIDs) == 0 {
		return nil, nil
	}

	var targets []DepartureTarget
	err := r.db.NewSelect().
		TableExpr(`"direction_stop" AS ds`).
		ColumnExpr(`s."id" AS "stopId", s."sourceId" AS "stopSourceId"`).
		ColumnExpr(`r."id" AS "routeId", r."sourceId" AS "routeSourceId"`).
		ColumnExpr(`d."id" AS "directionId", d."sourceId" AS "directionSourceId"`).
		Join(`JOIN "stop" AS s ON s."id" = ds."stopId"`).
		Join(`JOIN "direction" AS d ON d."id" = ds."directionId"`).
		Join(`JOIN "route" AS r ON r."id" = d."routeId"`).
		Where("r.active = TRUE").
		Where(`r."id" IN (?)`, bun.In(routeIDs)).
		Scan(ctx, &targets)

	return targets, err
}

// GetSubscribedRouteIDs leaves out subscriptions not refreshed within staleAfter.
func (r *RouteDataRepository) GetSubscribedRouteIDs(ctx context.Context, staleAfter time.Duration) ([]uuid.UUID, error) {
	var routeIDs []uuid.UUID
	err := r.db.NewSelect().
		Model((*model.LiveDataSubscription)(nil)).
		ColumnExpr(`DISTINCT ?`, bun.Ident("routeId")).
		Where(`? >= NOW() - make_interval(secs => ?)`, bun.Ident("lastUpdatedAt"), staleAfter.Seconds()).
		Scan(ctx, &routeIDs)

	return routeIDs, err
}

func (r *RouteDataRepository) DeleteStaleSubscriptions(ctx context.Context, staleAfter time.Duration) (int64, error) {
	res, err := r.db.NewDelete().
		Model((*model.LiveDataSubscription)(nil)).
		Where(`? < NOW() - make_interval(secs => ?)`, bun.Ident("lastUpdatedAt"), staleAfter.Seconds()).
		Exec(ctx)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (r *RouteDataRepository) MarkLiveDataAvailable(ctx context.Context, routeID uuid.UUID) error {
	_, err := r.db.NewUpdate().
		Model((*model.Route)(nil)).
		Set("? = TRUE", bun.Ident("liveDataAvailable")).
		Where("? = ?", bun.Ident("id"), routeID).
		Exec(ctx)
	return err
}

func (r *RouteDataRepository) UpsertRoutes(ctx context.Context, routes []model.Route) (map[string]model.Route, error) {
	return upsertAll(ctx, r.db, routes,
		func(route model.Route) string { return route.SourceID },
		upsertSpec{
			columns:  []string{"sourceId", "shortName", "longName", "lightColor", "darkColor"},
			conflict: `CONFLICT ("shortName") DO UPDATE`,
			set:      `"sourceId" = EXCLUDED."sourceId", "longName" = EXCLUDED."longName", "lightColor" = EXCLUDED."lightColor", "darkColor" = EXCLUDED."darkColor", "active" = TRUE, "deactivatedAt" = NULL`,
		})
}

func (r *RouteDataRepository) UpsertDirections(ctx context.Context, directions []model.Direction) (map[string]model.Direction, error) {
	return upsertAll(ctx, r.db, directions,
		func(direction model.Direction) string { return direction.SourceID },
		upsertSpec{
			columns:  []string{"sourceId", "routeId", "destination", "sequence", "path"},
			conflict: `CONFLICT ("routeId", "destination") DO UPDATE`,
			set:      `"sourceId" = EXCLUDED."sourceId", "sequence" = EXCLUDED."sequence", "path" = EXCLUDED."path"`,
		})
}

func (r *RouteDataRepository) UpsertStops(ctx context.Context, stops []model.Stop) (map[string]model.Stop, error) {
	for i := range stops {
		if stops[i].Amenities == nil {
			stops[i].Amenities = []string{}
		}
	}

	return upsertAll(ctx, r.db, stops,
		func(stop model.Stop) string { return stop.SourceID },
		upsertSpec{
			columns:  []string{"sourceId", "name", "lat", "lon", "amenities"},
			conflict: `CONFLICT ("sourceId") DO UPDATE`,
			set:      `"name" = EXCLUDED."name", "lat" = EXCLUDED."lat", "lon" = EXCLUDED."lon"`,
		})
}

func (r *RouteDataRepository) UpsertDirectionStops(ctx context.Context, directionStops []model.DirectionStop) error {
	_, err := upsertAll(ctx, r.db, directionStops,
		func(ds model.DirectionStop) string { return ds.DirectionID.String() + ds.StopID.String() },
		upsertSpec{
			columns:  []string{"directionId", "stopId", "sequence"},
			conflict: `CONFLICT ("directionId", "stopId") DO UPDATE`,
			set:      `"sequence" = EXCLUDED."sequence"`,
		})

	return err
}

// SyncVehicles replaces the vehicles of the given routes, leaving other routes untouched.
func (r *RouteDataRepository) SyncVehicles(ctx context.Context, routeIDs []uuid.UUID, vehicles []model.Vehicle) error {
	if len(routeIDs) == 0 {
		return nil
	}

	for i := range vehicles {
		if vehicles[i].Amenities == nil {
			vehicles[i].Amenities = []string{}
		}
	}

	return r.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		stored, err := upsertAll(ctx, tx, vehicles,
			func(vehicle model.Vehicle) string { return vehicle.SourceID },
			upsertSpec{
				columns:  []string{"sourceId", "routeId", "directionId", "name", "lat", "lon", "heading", "speed", "passengers", "capacity", "amenities"},
				conflict: `CONFLICT ("sourceId") DO UPDATE`,
				set:      `"routeId" = EXCLUDED."routeId", "directionId" = EXCLUDED."directionId", "name" = EXCLUDED."name", "lat" = EXCLUDED."lat", "lon" = EXCLUDED."lon", "heading" = EXCLUDED."heading", "speed" = EXCLUDED."speed", "passengers" = EXCLUDED."passengers", "capacity" = EXCLUDED."capacity", "amenities" = EXCLUDED."amenities", "seenAt" = clock_timestamp()`,
			})
		if err != nil {
			return err
		}

		staleVehicles := tx.NewDelete().
			Model(&model.Vehicle{}).
			Where("? IN (?)", bun.Ident("routeId"), bun.In(routeIDs))

		if len(stored) > 0 {
			staleVehicles = staleVehicles.Where("? NOT IN (?)", bun.Ident("sourceId"), bun.In(lo.Keys(stored)))
		}

		_, err = staleVehicles.Exec(ctx)
		return err
	})
}

// SyncAlerts replaces every stored alert with the given set.
func (r *RouteDataRepository) SyncAlerts(ctx context.Context, alerts []model.Alert) (map[string]model.Alert, error) {
	var stored map[string]model.Alert

	err := r.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		var err error
		stored, err = upsertAll(ctx, tx, alerts,
			func(alert model.Alert) string { return alert.SourceID },
			upsertSpec{
				columns:  []string{"sourceId", "title", "description", "timeRangeText", "dailyStartTime", "dailyEndTime", "startsAt", "endsAt"},
				conflict: `CONFLICT ("sourceId") DO UPDATE`,
				set:      `"title" = EXCLUDED."title", "description" = EXCLUDED."description", "timeRangeText" = EXCLUDED."timeRangeText", "dailyStartTime" = EXCLUDED."dailyStartTime", "dailyEndTime" = EXCLUDED."dailyEndTime", "startsAt" = EXCLUDED."startsAt", "endsAt" = EXCLUDED."endsAt"`,
			})
		if err != nil {
			return err
		}

		staleAlerts := tx.NewDelete().Model(&model.Alert{})
		if len(stored) == 0 {
			staleAlerts = staleAlerts.Where("TRUE")
		} else {
			staleAlerts = staleAlerts.Where("? NOT IN (?)", bun.Ident("sourceId"), bun.In(lo.Keys(stored)))
		}

		_, err = staleAlerts.Exec(ctx)
		return err
	})
	if err != nil {
		return nil, err
	}

	return stored, nil
}

// SyncAlertDirections replaces every stored alert direction with the given set.
func (r *RouteDataRepository) SyncAlertDirections(ctx context.Context, alertDirections []model.AlertDirection) error {
	return r.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		stored, err := upsertAll(ctx, tx, alertDirections,
			func(ad model.AlertDirection) string { return ad.AlertID.String() + ad.DirectionID.String() },
			upsertSpec{
				columns:  []string{"alertId", "directionId"},
				conflict: `CONFLICT ("alertId", "directionId") DO UPDATE`,
				// a no-op update, so RETURNING hands back the ids of rows that already existed
				set: `"alertId" = EXCLUDED."alertId"`,
			})
		if err != nil {
			return err
		}

		staleAlertDirections := tx.NewDelete().Model(&model.AlertDirection{})
		if len(stored) == 0 {
			staleAlertDirections = staleAlertDirections.Where("TRUE")
		} else {
			storedIDs := lo.MapToSlice(stored, func(_ string, ad model.AlertDirection) uuid.UUID { return ad.ID })
			staleAlertDirections = staleAlertDirections.Where("? NOT IN (?)", bun.Ident("id"), bun.In(storedIDs))
		}

		_, err = staleAlertDirections.Exec(ctx)
		return err
	})
}

func (r *RouteDataRepository) GetStops(ctx context.Context) ([]model.Stop, error) {
	var stops []model.Stop
	err := r.db.NewSelect().Model(&stops).Scan(ctx)

	return stops, err
}

type stopAmenities struct {
	SourceID  string   `json:"sourceId"`
	Amenities []string `json:"amenities"`
}

func (r *RouteDataRepository) UpdateStopAmenities(ctx context.Context, amenities map[string][]string) error {
	if len(amenities) == 0 {
		return nil
	}

	rows := lo.MapToSlice(amenities, func(sourceID string, names []string) stopAmenities {
		if names == nil {
			names = []string{}
		}
		return stopAmenities{SourceID: sourceID, Amenities: names}
	})

	payload, err := json.Marshal(rows)
	if err != nil {
		return err
	}

	_, err = r.db.NewUpdate().
		Model((*model.Stop)(nil)).
		TableExpr(`jsonb_to_recordset(?::jsonb) AS data("sourceId" text, "amenities" jsonb)`, string(payload)).
		Set(`"amenities" = data."amenities"`).
		Where(`"stop"."sourceId" = data."sourceId"`).
		Exec(ctx)

	return err
}

// SyncDepartures replaces the departures of the given routes at the given stops,
// leaving other routes and stops that were not fetched this round untouched.
func (r *RouteDataRepository) SyncDepartures(ctx context.Context, routeIDs []uuid.UUID, stopIDs []uuid.UUID, departures []model.Departure) error {
	if len(routeIDs) == 0 || len(stopIDs) == 0 {
		return nil
	}

	return r.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		stored, err := upsertAll(ctx, tx, departures,
			func(d model.Departure) string {
				return d.RouteID.String() + d.StopID.String() + d.DirectionID.String() + d.ScheduledAt.String()
			},
			upsertSpec{
				columns:  []string{"routeId", "stopId", "directionId", "scheduledAt", "estimatedAt", "isCancelled"},
				conflict: `CONFLICT ("routeId", "stopId", "directionId", "scheduledAt") DO UPDATE`,
				set:      `"estimatedAt" = EXCLUDED."estimatedAt", "isCancelled" = EXCLUDED."isCancelled"`,
			})
		if err != nil {
			return err
		}

		staleDepartures := tx.NewDelete().
			Model(&model.Departure{}).
			Where("? IN (?)", bun.Ident("stopId"), bun.In(stopIDs)).
			Where("? IN (?)", bun.Ident("routeId"), bun.In(routeIDs))

		if len(stored) > 0 {
			storedIDs := lo.MapToSlice(stored, func(_ string, d model.Departure) uuid.UUID { return d.ID })
			staleDepartures = staleDepartures.Where("? NOT IN (?)", bun.Ident("id"), bun.In(storedIDs))
		}

		_, err = staleDepartures.Exec(ctx)
		return err
	})
}

func (r *RouteDataRepository) DeactivateInactiveRoutes(ctx context.Context, activeRoutes []string) error {
	if len(activeRoutes) == 0 {
		return nil
	}

	_, err := r.db.NewUpdate().
		Model(&model.Route{}).
		Set("active = false, ? = NOW()", bun.Ident("deactivatedAt")).
		Where("? NOT IN (?)", bun.Ident("sourceId"), bun.In(activeRoutes)).
		Where("active = TRUE").
		Exec(ctx)

	return err
}

func (r *RouteDataRepository) CleanupInactiveRoutes(ctx context.Context, inactiveDuration time.Duration) error {
	_, err := r.db.NewDelete().
		Model(&model.Route{}).
		Where("active = false AND ? < ?", bun.Ident("deactivatedAt"), time.Now().Add(-inactiveDuration)).
		Exec(ctx)

	return err
}

func (r *RouteDataRepository) CleanupSyncTombstones(ctx context.Context, inactiveDuration time.Duration) error {
	cutoff := time.Now().Add(-inactiveDuration)

	for _, table := range model.SyncTables {
		_, err := r.db.NewDelete().
			Model(table.AuditModel).
			Where("? < ?", bun.Ident("deletedAt"), cutoff).
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("cleanup %s: %w", table.Audit, err)
		}
	}

	return nil
}
