package jobs

import (
	"context"
	"log/slog"
	"time"

	"github.com/go-co-op/gocron/v2"

	"github.com/MaroonRides/api/apps/requester/repositories"
	"github.com/MaroonRides/api/internal/db/sync"
)

const InactiveRouteRetention = 14 * 24 * time.Hour

func NewDatabaseCleanupJob(repo *repositories.RouteDataRepository) Job {
	return Job{
		Task: func(ctx context.Context) error {

			err := repo.CleanupInactiveRoutes(ctx, InactiveRouteRetention)

			if err != nil {
				slog.Error("failed to cleanup inactive routes", "error", err)
				return err
			}

			err = repo.CleanupSyncTombstones(ctx, sync.TombstoneRetention)

			if err != nil {
				slog.Error("failed to cleanup sync tombstones", "error", err)
				return err
			}

			return nil
		},
		Schedule: gocron.DurationJob(12 * time.Hour),
		Name:     JobDatabaseCleanup,
	}
}
