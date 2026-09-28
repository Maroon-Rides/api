package jobs

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	_ "time/tzdata"

	"github.com/go-co-op/gocron/v2"
	"go.uber.org/fx"
)

// ServiceTimeZone is where the buses run. The alpine image ships no zoneinfo,
// hence the embedded tzdata.
const ServiceTimeZone = "America/Chicago"

type Job struct {
	Task     func(context.Context) error
	Schedule gocron.JobDefinition
	Name     string
}

type scheduledJob struct {
	job    Job
	handle gocron.Job
}

type Scheduler struct {
	Scheduler gocron.Scheduler

	mu   sync.Mutex
	jobs map[string]scheduledJob
}

func jobOptions() []gocron.JobOption {
	return []gocron.JobOption{
		gocron.WithSingletonMode(gocron.LimitModeReschedule),
		gocron.WithStartAt(gocron.WithStartImmediately()),
	}
}

func NewServiceLocation() (*time.Location, error) {
	location, err := time.LoadLocation(ServiceTimeZone)
	if err != nil {
		return nil, fmt.Errorf("loading %s: %w", ServiceTimeZone, err)
	}
	return location, nil
}

func NewScheduler(location *time.Location) (*Scheduler, error) {
	s, err := gocron.NewScheduler(gocron.WithLocation(location))
	if err != nil {
		return nil, err
	}

	return &Scheduler{
		Scheduler: s,
		jobs:      map[string]scheduledJob{},
	}, nil
}

func (s *Scheduler) Register(jobs []Job) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, job := range jobs {
		if _, exists := s.jobs[job.Name]; exists {
			return fmt.Errorf("duplicate job name %q", job.Name)
		}

		handle, err := s.Scheduler.NewJob(
			job.Schedule,
			gocron.NewTask(job.Task),
			jobOptions()...,
		)
		if err != nil {
			return fmt.Errorf("scheduling job %q: %w", job.Name, err)
		}

		s.jobs[job.Name] = scheduledJob{job: job, handle: handle}
	}

	return nil
}

// RunNow starts the job immediately and pushes the next run a full interval out.
func (s *Scheduler) RunNow(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	scheduled, ok := s.jobs[name]
	if !ok {
		return fmt.Errorf("no job named %q", name)
	}

	handle, err := s.Scheduler.Update(
		scheduled.handle.ID(),
		scheduled.job.Schedule,
		gocron.NewTask(scheduled.job.Task),
		jobOptions()...,
	)
	if err != nil {
		return fmt.Errorf("rescheduling job %q: %w", name, err)
	}

	scheduled.handle = handle
	s.jobs[name] = scheduled
	return nil
}

func (s *Scheduler) StartScheduler(lc fx.Lifecycle) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			slog.Info("Job scheduler started")
			s.Scheduler.Start()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			return s.Scheduler.Shutdown()
		},
	})
}
