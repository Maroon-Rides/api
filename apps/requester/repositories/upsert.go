package repositories

import (
	"context"

	"github.com/samber/lo"
	"github.com/uptrace/bun"
)

type upsertSpec struct {
	columns  []string
	conflict string
	set      string
}

func upsertAll[T any](ctx context.Context, db *bun.DB, rows []T, key func(T) string, spec upsertSpec) (map[string]T, error) {
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
