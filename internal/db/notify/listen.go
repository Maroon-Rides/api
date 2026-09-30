package notify

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/uptrace/bun"
	"go.uber.org/fx"
)

const listenRetryDelay = 5 * time.Second

// Handlers maps a channel to the function that receives its payloads.
type Handlers map[string]func(ctx context.Context, payload string) error

// ListenOnStart holds one connection listening on every channel in handlers for the app's lifetime.
func ListenOnStart(lc fx.Lifecycle, db *bun.DB, handlers Handlers) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			go func() {
				defer close(done)
				listenUntilDone(ctx, db, handlers)
			}()
			return nil
		},
		OnStop: func(stopCtx context.Context) error {
			cancel()
			select {
			case <-done:
				return nil
			case <-stopCtx.Done():
				return stopCtx.Err()
			}
		},
	})
}

func listenUntilDone(ctx context.Context, db *bun.DB, handlers Handlers) {
	for {
		err := listen(ctx, db, handlers)
		if ctx.Err() != nil {
			return
		}
		slog.Error("Database listener stopped, retrying", "error", err)

		select {
		case <-ctx.Done():
			return
		case <-time.After(listenRetryDelay):
		}
	}
}

func listen(ctx context.Context, db *bun.DB, handlers Handlers) error {
	conn, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("acquire connection: %w", err)
	}
	defer conn.Close()

	return conn.Raw(func(driverConn any) error {
		pg := driverConn.(*stdlib.Conn).Conn()
		defer unlisten(pg.PgConn())

		for channel := range handlers {
			if _, err := pg.Exec(ctx, "LISTEN "+pgx.Identifier{channel}.Sanitize()); err != nil {
				return fmt.Errorf("listen on %s: %w", channel, err)
			}
		}

		for {
			n, err := pg.WaitForNotification(ctx)
			if err != nil {
				return fmt.Errorf("wait for notification: %w", err)
			}
			if err := handlers[n.Channel](ctx, n.Payload); err != nil {
				slog.Error("Failed to handle notification", "channel", n.Channel, "payload", n.Payload, "error", err)
			}
		}
	})
}

// The connection goes back to the pool, so it must stop receiving notifications.
func unlisten(pg *pgconn.PgConn) {
	if pg.IsClosed() {
		return
	}
	if err := pg.Exec(context.Background(), "UNLISTEN *").Close(); err != nil {
		slog.Error("Failed to unlisten", "error", err)
	}
}
