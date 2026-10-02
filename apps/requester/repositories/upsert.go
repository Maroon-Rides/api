package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/uptrace/bun"
)

type upsertSpec struct {
	columns  []string
	conflict string
	set      string
}

func upsertAll[T any](ctx context.Context, db bun.IDB, rows []T, key func(T) string, spec upsertSpec) (map[string]T, error) {
	rows = lo.UniqBy(rows, key)

	if len(rows) == 0 {
		return map[string]T{}, nil
	}

	_, err := db.NewInsert().
		Model(&rows).
		Column(spec.columns...).
		On(spec.conflict).
		Set(spec.set).
		Returning("*").
		Exec(ctx)
	if err != nil {
		return nil, err
	}

	stored := lo.KeyBy(rows, key)
	return stored, nil
}

// syncScope narrows a delete to the stored rows a sync replaces.
type syncScope func(*bun.DeleteQuery) *bun.DeleteQuery

func everyRow(q *bun.DeleteQuery) *bun.DeleteQuery {
	return q.Where("TRUE")
}

func whereIn[V any](column string, values []V) syncScope {
	return func(q *bun.DeleteQuery) *bun.DeleteQuery {
		return q.Where("? IN (?)", bun.Ident(column), bun.In(values))
	}
}

// syncAll upserts rows, then deletes every row in scope the upsert did not touch.
func syncAll[T any](
	ctx context.Context,
	db *bun.DB,
	rows []T,
	key func(T) string,
	id func(T) uuid.UUID,
	spec upsertSpec,
	scopes ...syncScope,
) (map[string]T, error) {
	var stored map[string]T

	err := db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		var err error
		stored, err = upsertAll(ctx, tx, rows, key, spec)
		if err != nil {
			return err
		}

		stale := tx.NewDelete().Model((*T)(nil))
		for _, scope := range scopes {
			stale = scope(stale)
		}

		if len(stored) > 0 {
			storedIDs := lo.MapToSlice(stored, func(_ string, row T) uuid.UUID { return id(row) })
			stale = stale.Where("? NOT IN (?)", bun.Ident("id"), bun.In(storedIDs))
		}

		_, err = stale.Exec(ctx)
		return err
	})
	if err != nil {
		return nil, err
	}

	return stored, nil
}
