package jobs

import (
	"context"
	"log/slog"
	"time"

	"github.com/MaroonRides/api/apps/requester/services"

	"github.com/go-co-op/gocron/v2"
)

func NewDatabaseCleanupJob(service *services.DatabaseCleanupService) Job {
	return Job{
		Task: func(ctx context.Context) error {
			err := service.Cleanup(ctx)
			if err != nil {
				slog.Error("Failed to clean up database", "error", err)
			}

			return err
		},
		Schedule: gocron.DurationJob(12 * time.Hour),
		Name:     JobDatabaseCleanup,
	}
}
