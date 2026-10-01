package jobs

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/MaroonRides/api/apps/requester/services"

	"github.com/go-co-op/gocron/v2"
)

func NewRouteKeysJob(service *services.RouteDataService, scheduler *Scheduler) Job {
	return Job{
		Task: func(ctx context.Context) error {
			synced, err := service.RouteKeysSynced(ctx)
			if err != nil {
				return err
			}
			if synced {
				return nil
			}
			slog.Warn("Route or direction keys have changed, triggering refresh of data.")
			if err := scheduler.RunNow(JobRouteData); err != nil {
				return fmt.Errorf("triggering route data refresh: %w", err)
			}
			return nil
		},
		Schedule: gocron.DurationJob(30 * time.Second),
		Name:     JobRouteKeys,
	}
}
