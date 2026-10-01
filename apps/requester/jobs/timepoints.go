package jobs

import (
	"time"

	"github.com/MaroonRides/api/apps/requester/services"

	"github.com/go-co-op/gocron/v2"
)

func NewTimepointsJob(service *services.TimepointService) Job {
	return Job{
		Task:     service.Sync,
		Schedule: gocron.DurationJob(1 * time.Hour),
		Name:     JobTimepoints,
	}
}
