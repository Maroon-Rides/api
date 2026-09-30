package jobs

import (
	"context"
	"log/slog"
	"time"

	"github.com/MaroonRides/api/apps/requester/services"

	"github.com/go-co-op/gocron/v2"
)

func NewDepartureTimesJob(service *services.LiveDataService) Job {
	return Job{
		Task: func(ctx context.Context) error {
			if err := service.SyncSubscribedDepartures(ctx); err != nil {
				slog.Error("Failed to sync departure times", "error", err)
			}

			return nil
		},
		Schedule: gocron.DurationJob(30 * time.Second),
		Name:     JobDepartureTimes,
	}
}
