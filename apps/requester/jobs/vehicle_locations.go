package jobs

import (
	"context"
	"log/slog"
	"time"

	"github.com/MaroonRides/api/apps/requester/services"

	"github.com/go-co-op/gocron/v2"
)

func NewVehicleLocationsJob(service *services.LiveDataService) Job {
	return Job{
		Task: func(ctx context.Context) error {
			if err := service.SyncActiveVehicles(ctx); err != nil {
				slog.Error("Failed to sync vehicle locations", "error", err)
			}

			return nil
		},
		Schedule: gocron.DurationJob(10 * time.Second),
		Name:     JobVehicleLocations,
	}
}
