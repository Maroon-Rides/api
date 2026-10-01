package jobs

import (
	"time"

	"github.com/MaroonRides/api/apps/requester/services"

	"github.com/go-co-op/gocron/v2"
)

func NewDepartureTimesJob(service *services.LiveDataService) Job {
	return Job{
		Task:     service.SyncSubscribedDepartures,
		Schedule: gocron.DurationJob(30 * time.Second),
		Name:     JobDepartureTimes,
	}
}
