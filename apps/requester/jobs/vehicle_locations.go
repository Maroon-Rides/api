package jobs

import (
	"time"

	"github.com/MaroonRides/api/apps/requester/services"

	"github.com/go-co-op/gocron/v2"
)

func NewVehicleLocationsJob(service *services.LiveDataService) Job {
	return Job{
		Task:     service.SyncActiveVehicles,
		Schedule: gocron.DurationJob(10 * time.Second),
		Name:     JobVehicleLocations,
	}
}
