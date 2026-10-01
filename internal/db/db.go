// Package db opens the application database and brings its schema up to date.
package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"go.uber.org/fx"

	"github.com/MaroonRides/api/internal/db/migrations"
	"github.com/MaroonRides/api/internal/db/model"
	"github.com/MaroonRides/api/internal/db/notify"
	"github.com/MaroonRides/api/internal/db/sync"
)

const (
	envDatabaseURL      = "DATABASE_URL"
	envDatabaseMaxConns = "DATABASE_MAX_CONNS"

	// Every replica of every app opens up to this many, and together they must stay under the server's max_connections.
	defaultMaxConns = 20
	connMaxIdleTime = 5 * time.Minute
)

var ErrDatabaseURLNotSet = errors.New(envDatabaseURL + " is not set")

var Module = fx.Provide(New)

// Migrated is a dependency for anything that must start after the schema is up to date.
type Migrated struct{}

// MigrateOnStart is opt-in so only one app owns schema changes.
var MigrateOnStart = fx.Provide(func(lc fx.Lifecycle, bundb *bun.DB) Migrated {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			return Migrate(ctx, bundb)
		},
	})
	return Migrated{}
})

func New(lc fx.Lifecycle) (*bun.DB, error) {
	dsn := os.Getenv(envDatabaseURL)
	if dsn == "" {
		return nil, ErrDatabaseURLNotSet
	}

	maxConns, err := maxConnsFromEnv()
	if err != nil {
		return nil, err
	}

	bundb, err := Open(dsn)
	if err != nil {
		return nil, err
	}
	bundb.SetMaxOpenConns(maxConns)
	bundb.SetMaxIdleConns(maxConns)
	bundb.SetConnMaxIdleTime(connMaxIdleTime)

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

func maxConnsFromEnv() (int, error) {
	raw := os.Getenv(envDatabaseMaxConns)
	if raw == "" {
		return defaultMaxConns, nil
	}

	maxConns, err := strconv.Atoi(raw)
	if err != nil || maxConns < 1 {
		return 0, fmt.Errorf("%s must be a positive integer, got %q", envDatabaseMaxConns, raw)
	}
	return maxConns, nil
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
	if err := notify.Reconcile(ctx, db); err != nil {
		return fmt.Errorf("reconcile notify triggers: %w", err)
	}
	return nil
}
