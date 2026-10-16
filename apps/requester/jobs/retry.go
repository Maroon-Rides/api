package jobs

import (
	"context"
	"time"

	"github.com/sethvargo/go-retry"

	"github.com/MaroonRides/api/apps/requester/metrics"
)

// Backoff waits 1s then 2s, so a run finishes retrying well inside the
// shortest job interval (vehicle locations, 10s).
const (
	retryAttempts  = 3
	retryBaseDelay = 1 * time.Second
	retryMaxDelay  = 4 * time.Second
)

type retryPolicy struct {
	attempts  uint64
	baseDelay time.Duration
	maxDelay  time.Duration
}

var defaultRetryPolicy = retryPolicy{
	attempts:  retryAttempts,
	baseDelay: retryBaseDelay,
	maxDelay:  retryMaxDelay,
}

func (p retryPolicy) backoff() retry.Backoff {
	return retry.WithMaxRetries(p.attempts-1, retry.WithCappedDuration(p.maxDelay, retry.NewExponential(p.baseDelay)))
}

func withRetries(policy retryPolicy, m *metrics.JobMetrics, name string, task func(context.Context) error) func(context.Context) error {
	attempt := m.Instrument(name, task)

	return func(ctx context.Context) error {
		err := retry.Do(ctx, policy.backoff(), func(ctx context.Context) error {
			return retry.RetryableError(attempt(ctx))
		})
		if err != nil && ctx.Err() == nil {
			m.RetriesExhausted(name)
		}
		return err
	}
}
