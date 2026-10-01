package jobs

import (
	"go.uber.org/fx"

	"github.com/MaroonRides/api/internal/db"
)

func AsJob(f any) any {
	return fx.Annotate(f, fx.ResultTags(`group:"jobs"`))
}

const (
	JobRouteData          = "route-data"
	JobVehicleLocations   = "vehicle-locations"
	JobDepartureTimes     = "departure-times"
	JobRouteKeys          = "route-keys"
	JobDatabaseCleanup    = "database-cleanup"
	JobTimetable          = "timetable"
	JobSubscriptionReaper = "subscription-reaper"
	JobTimepoints         = "timepoints"
)

var Module = fx.Options(
	fx.Provide(
		AsJob(NewRouteDataJob),
		AsJob(NewVehicleLocationsJob),
		AsJob(NewDepartureTimesJob),
		AsJob(NewRouteKeysJob),
		AsJob(NewDatabaseCleanupJob),
		AsJob(NewTimetableJob),
		AsJob(NewSubscriptionReaperJob),
		AsJob(NewTimepointsJob),
	),
	fx.Provide(NewScheduler),
	fx.Invoke(fx.Annotate(
		func(lc fx.Lifecycle, scheduler *Scheduler, jobs []Job, _ db.Migrated) error {
			if err := scheduler.Register(jobs); err != nil {
				return err
			}
			scheduler.StartScheduler(lc)
			return nil
		},
		fx.ParamTags(``, ``, `group:"jobs"`),
	)),
)
