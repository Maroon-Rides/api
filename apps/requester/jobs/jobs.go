package jobs

import (
	"go.uber.org/fx"
)

func AsJob(f any) any {
	return fx.Annotate(f, fx.ResultTags(`group:"jobs"`))
}

const (
	JobRouteData       = "route-data"
	JobLiveData        = "live-data"
	JobDatabaseCleanup = "database-cleanup"
	JobTimetable       = "timetable"
)

var Module = fx.Options(
	fx.Provide(
		AsJob(NewRouteDataJob),
		AsJob(NewLiveDataJob),
		AsJob(NewDatabaseCleanupJob),
		AsJob(NewTimetableJob),
	),
	fx.Provide(NewServiceLocation, NewScheduler),
	fx.Invoke(fx.Annotate(
		func(lc fx.Lifecycle, scheduler *Scheduler, jobs []Job) error {
			if err := scheduler.Register(jobs); err != nil {
				return err
			}
			scheduler.StartScheduler(lc)
			return nil
		},
		fx.ParamTags(``, ``, `group:"jobs"`),
	)),
)
