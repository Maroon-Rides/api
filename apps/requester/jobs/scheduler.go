package jobs

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/go-co-op/gocron/v2"
	"github.com/google/uuid"
	"go.uber.org/fx"

	"github.com/MaroonRides/api/apps/requester/metrics"
)

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
	metrics   *metrics.JobMetrics

	mu   sync.Mutex
	jobs map[string]scheduledJob
}

func jobOptions(name string) []gocron.JobOption {
	return []gocron.JobOption{
		gocron.WithName(name),
		gocron.WithSingletonMode(gocron.LimitModeReschedule),
		gocron.WithStartAt(gocron.WithStartImmediately()),
		gocron.WithEventListeners(gocron.AfterJobRunsWithError(logJobFailure)),
	}
}

func logJobFailure(_ uuid.UUID, name string, err error) {
	slog.Error("Job failed", "job", name, "error", err)
}

func NewScheduler(location *time.Location, metrics *metrics.JobMetrics) (*Scheduler, error) {
	s, err := gocron.NewScheduler(gocron.WithLocation(location))
	if err != nil {
		return nil, err
	}

	return &Scheduler{
		Scheduler: s,
		metrics:   metrics,
		jobs:      map[string]scheduledJob{},
	}, nil
}

func (s *Scheduler) task(job Job) gocron.Task {
	return gocron.NewTask(s.metrics.Instrument(job.Name, job.Task))
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
			s.task(job),
			jobOptions(job.Name)...,
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
		s.task(scheduled.job),
		jobOptions(scheduled.job.Name)...,
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
