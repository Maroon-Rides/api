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

// SyncStopSchedules replaces one service date's schedules for the given stops,
// leaving stops that were not fetched untouched.
func (r *TimetableRepository) SyncStopSchedules(ctx context.Context, serviceDate time.Time, stopIDs []uuid.UUID, schedules []model.StopSchedule) error {
	if len(stopIDs) == 0 {
		return nil
	}

	stored, err := upsertAll(ctx, r.db, schedules,
		func(s model.StopSchedule) string {
			return s.StopID.String() + s.DirectionID.String() + s.ScheduledAt.String()
		},
		upsertSpec{
			columns:  []string{"stopId", "directionId", "scheduledAt", "serviceDate"},
			conflict: `CONFLICT ("stopId", "directionId", "scheduledAt") DO UPDATE`,
			set:      `"serviceDate" = EXCLUDED."serviceDate"`,
		})
	if err != nil {
		return err
	}

	staleSchedules := r.db.NewDelete().
		Model(&model.StopSchedule{}).
		Where("? = ?", bun.Ident("serviceDate"), serviceDate.Format(time.DateOnly)).
		Where("? IN (?)", bun.Ident("stopId"), bun.In(stopIDs))

	if len(stored) > 0 {
		storedIDs := lo.MapToSlice(stored, func(_ string, s model.StopSchedule) uuid.UUID { return s.ID })
		staleSchedules = staleSchedules.Where("? NOT IN (?)", bun.Ident("id"), bun.In(storedIDs))
	}

	_, err = staleSchedules.Exec(ctx)
	return err
}

func (r *TimetableRepository) DeleteStopSchedulesBefore(ctx context.Context, serviceDate time.Time) error {
	_, err := r.db.NewDelete().
		Model(&model.StopSchedule{}).
		Where("? < ?", bun.Ident("serviceDate"), serviceDate.Format(time.DateOnly)).
		Exec(ctx)

	return err
}
