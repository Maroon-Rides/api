// Package db opens the application database and brings its schema up to date.
package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"go.uber.org/fx"

	"github.com/MaroonRides/api/internal/db/migrations"
	"github.com/MaroonRides/api/internal/db/model"
	"github.com/MaroonRides/api/internal/db/sync"
)

const envDatabaseURL = "DATABASE_URL"

var ErrDatabaseURLNotSet = errors.New(envDatabaseURL + " is not set")

var Module = fx.Provide(New)

// MigrateOnStart is opt-in so only one app owns schema changes.
var MigrateOnStart = fx.Invoke(func(lc fx.Lifecycle, bundb *bun.DB) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			return Migrate(ctx, bundb)
		},
	})
})

func New(lc fx.Lifecycle) (*bun.DB, error) {
	dsn := os.Getenv(envDatabaseURL)
	if dsn == "" {
		return nil, ErrDatabaseURLNotSet
	}

	bundb, err := Open(dsn)
	if err != nil {
		return nil, err
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			return bundb.PingContext(ctx)
		},
		OnStop: func(ctx context.Context) error {
			return bundb.Close()
		},
	})

	return bundb, nil
}

func Open(dsn string) (*bun.DB, error) {
	sqldb, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	return bun.NewDB(sqldb, pgdialect.New()), nil
}

// Migrate applies the goose migrations, then reconciles the trigger layer. The
// triggers live outside the migrations because atlas OSS does not inspect them
// -- a declarative round-trip would silently drop them.
func Migrate(ctx context.Context, db *bun.DB) error {
	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("set goose dialect: %w", err)
	}
	if err := goose.UpContext(ctx, db.DB, "."); err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}
	if err := sync.Reconcile(ctx, db, model.SyncTables); err != nil {
		return fmt.Errorf("reconcile sync triggers: %w", err)
	}
	return nil
}
