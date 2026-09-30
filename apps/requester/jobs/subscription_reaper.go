package jobs

import (
	"context"
	"log/slog"

	"github.com/MaroonRides/api/apps/requester/services"
	"github.com/MaroonRides/api/internal/db/model"

	"github.com/go-co-op/gocron/v2"
)

func NewSubscriptionReaperJob(service *services.LiveDataService) Job {
	return Job{
		Task: func(ctx context.Context) error {
			if err := service.ReapStaleSubscriptions(ctx); err != nil {
				slog.Error("Failed to reap stale subscriptions", "error", err)
			}

			return nil
		},
		Schedule: gocron.DurationJob(model.LiveDataSubscriptionStaleAfter),
		Name:     JobSubscriptionReaper,
	}
}
