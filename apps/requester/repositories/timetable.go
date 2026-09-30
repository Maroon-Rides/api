package repositories

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/uptrace/bun"

	"github.com/MaroonRides/api/internal/db/model"
)

type TimetableRepository struct {
	db *bun.DB
}

func NewTimetableRepository(db *bun.DB) *TimetableRepository {
	return &TimetableRepository{db: db}
}

// SyncTimetables replaces one service date's timetables for the given stops,
// leaving stops that were not fetched untouched.
func (r *TimetableRepository) SyncTimetables(ctx context.Context, serviceDate time.Time, stopIDs []uuid.UUID, timetables []model.Timetable) error {
	if len(stopIDs) == 0 {
		return nil
	}

	return r.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		stored, err := upsertAll(ctx, tx, timetables,
			func(t model.Timetable) string {
				return t.StopID.String() + t.DirectionID.String() + t.ServiceDate.Format(time.DateOnly)
			},
			upsertSpec{
				columns:  []string{"stopId", "directionId", "routeId", "serviceDate", "departures"},
				conflict: `CONFLICT ("stopId", "directionId", "serviceDate") DO UPDATE`,
				set:      `"departures" = EXCLUDED."departures"`,
			})
		if err != nil {
			return err
		}

		staleTimetables := tx.NewDelete().
			Model(&model.Timetable{}).
			Where("? = ?", bun.Ident("serviceDate"), serviceDate.Format(time.DateOnly)).
			Where("? IN (?)", bun.Ident("stopId"), bun.In(stopIDs))

		if len(stored) > 0 {
			storedIDs := lo.MapToSlice(stored, func(_ string, t model.Timetable) uuid.UUID { return t.ID })
			staleTimetables = staleTimetables.Where("? NOT IN (?)", bun.Ident("id"), bun.In(storedIDs))
		}

		_, err = staleTimetables.Exec(ctx)
		return err
	})
}

func (r *TimetableRepository) DeleteTimetablesBefore(ctx context.Context, serviceDate time.Time) error {
	_, err := r.db.NewDelete().
		Model(&model.Timetable{}).
		Where("? < ?", bun.Ident("serviceDate"), serviceDate.Format(time.DateOnly)).
		Exec(ctx)

	return err
}
