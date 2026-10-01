package jobs

import (
	"github.com/MaroonRides/api/apps/requester/services"
	"github.com/MaroonRides/api/internal/db/model"

	"github.com/go-co-op/gocron/v2"
)

func NewSubscriptionReaperJob(service *services.LiveDataService) Job {
	return Job{
		Task:     service.ReapStaleSubscriptions,
		Schedule: gocron.DurationJob(model.LiveDataSubscriptionStaleAfter),
		Name:     JobSubscriptionReaper,
	}
}
