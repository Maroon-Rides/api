package jobs

import (
	"github.com/MaroonRides/api/apps/requester/services"

	"github.com/go-co-op/gocron/v2"
)

const timetableSyncHour = 2

func NewTimetableJob(service *services.TimetableService) Job {
	return Job{
		Task:     service.Sync,
		Schedule: gocron.DailyJob(1, gocron.NewAtTimes(gocron.NewAtTime(timetableSyncHour, 0, 0))),
		Name:     JobTimetable,
	}
}
