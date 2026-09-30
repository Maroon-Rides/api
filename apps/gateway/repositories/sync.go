package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"iter"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"github.com/MaroonRides/api/internal/db/model"
	"github.com/MaroonRides/api/internal/db/sync"
)

// Rows minted within this window may still sit in an uncommitted write, so a
// stream leaves them for the next sync.
const NowIDLag = time.Millisecond

type SyncRepository struct {
	db *bun.DB
}

func NewSyncRepository(db *bun.DB) *SyncRepository {
	return &SyncRepository{db: db}
}

func (r *SyncRepository) NowID(ctx context.Context) (uuid.UUID, error) {
	var id uuid.UUID
	err := r.db.NewRaw("SELECT uuidv7(-make_interval(secs => ?))", NowIDLag.Seconds()).Scan(ctx, &id)
	if err != nil {
		return uuid.Nil, fmt.Errorf("mint now id: %w", err)
	}
	return id, nil
}

// ResetBefore returns the reset floor, or a nil uuid when none is set.
func (r *SyncRepository) ResetBefore(ctx context.Context) (uuid.UUID, error) {
	var meta model.SyncMetadata
	err := r.db.NewSelect().Model(&meta).Where(`"key" = ?`, model.SyncMetadataKeys.ResetBefore).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return uuid.Nil, nil
	}
	if err != nil {
		return uuid.Nil, fmt.Errorf("read reset floor: %w", err)
	}
	return meta.Value, nil
}

// Scope limits a query to the rows whose Column holds ID. The zero Scope reads every row.
type Scope struct {
	Column string
	ID     uuid.UUID
}

// Upserts yields rows of M changed after `after` and before `before`, oldest first.
// A nil `after` starts from the beginning.
func Upserts[M sync.Row](ctx context.Context, r *SyncRepository, scope Scope, after, before uuid.UUID) iter.Seq2[M, error] {
	return rowsBetween[M](ctx, r.db, scope, sync.UpsertColumn, after, before)
}

// Deletes yields tombstones of A written after `after` and before `before`, oldest first.
func Deletes[A sync.Row](ctx context.Context, r *SyncRepository, scope Scope, after, before uuid.UUID) iter.Seq2[A, error] {
	return rowsBetween[A](ctx, r.db, scope, sync.DeleteColumn, after, before)
}

func rowsBetween[M sync.Row](ctx context.Context, db *bun.DB, scope Scope, column string, after, before uuid.UUID) iter.Seq2[M, error] {
	return func(yield func(M, error) bool) {
		var zero M
		q := db.NewSelect().
			Model(&zero).
			Where("? < ?", bun.Ident(column), before).
			OrderExpr("? ASC", bun.Ident(column))
		if after != uuid.Nil {
			q = q.Where("? > ?", bun.Ident(column), after)
		}
		if scope.Column != "" {
			q = q.Where("? = ?", bun.Ident(scope.Column), scope.ID)
		}

		rows, err := q.Rows(ctx)
		if err != nil {
			yield(zero, fmt.Errorf("query %T: %w", zero, err))
			return
		}
		defer rows.Close()

		for rows.Next() {
			var row M
			if err := db.ScanRow(ctx, rows, &row); err != nil {
				yield(zero, fmt.Errorf("scan %T: %w", zero, err))
				return
			}
			if !yield(row, nil) {
				return
			}
		}
		if err := rows.Err(); err != nil {
			yield(zero, fmt.Errorf("read %T: %w", zero, err))
		}
	}
}
