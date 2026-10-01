package jobs

import (
	"time"

	"github.com/MaroonRides/api/apps/requester/services"

	"github.com/go-co-op/gocron/v2"
)

func NewDatabaseCleanupJob(service *services.DatabaseCleanupService) Job {
	return Job{
		Task:     service.Cleanup,
		Schedule: gocron.DurationJob(12 * time.Hour),
		Name:     JobDatabaseCleanup,
	}
}
