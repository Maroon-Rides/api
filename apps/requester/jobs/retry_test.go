package jobs

import (
	"context"
	"errors"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/MaroonRides/api/apps/requester/metrics"
)

const testJob = "test-job"

var fastRetryPolicy = retryPolicy{
	attempts:  3,
	baseDelay: time.Millisecond,
	maxDelay:  time.Millisecond,
}

func failsThenSucceeds(errs ...error) func(context.Context) error {
	return func(context.Context) error {
		if len(errs) == 0 {
			return nil
		}
		err := errs[0]
		errs = errs[1:]
		return err
	}
}

var _ = Describe("withRetries", Label("unit"), func() {
	var (
		registry *prometheus.Registry
		m        *metrics.JobMetrics
	)

	BeforeEach(func() {
		registry = prometheus.NewRegistry()
		m = metrics.NewJobMetrics(registry)
	})

	expectMetrics := func(expected string) {
		Expect(testutil.GatherAndCompare(registry, strings.NewReader(expected),
			"requester_job_runs_total", "requester_job_retries_exhausted_total")).To(Succeed())
	}

	It("runs once when the first attempt succeeds", func() {
		Expect(withRetries(fastRetryPolicy, m, testJob, failsThenSucceeds())(context.Background())).To(Succeed())

		expectMetrics(`
# HELP requester_job_runs_total Scheduled job attempts, by job and result. A retried run counts once per attempt.
# TYPE requester_job_runs_total counter
requester_job_runs_total{job="test-job",result="success"} 1
`)
	})

	It("succeeds after a failed attempt", func() {
		Expect(withRetries(fastRetryPolicy, m, testJob, failsThenSucceeds(errors.New("flaky")))(context.Background())).To(Succeed())

		expectMetrics(`
# HELP requester_job_runs_total Scheduled job attempts, by job and result. A retried run counts once per attempt.
# TYPE requester_job_runs_total counter
requester_job_runs_total{job="test-job",result="failure"} 1
requester_job_runs_total{job="test-job",result="success"} 1
`)
	})

	It("returns the last error and counts the run as exhausted when every attempt fails", func() {
		last := errors.New("third")
		task := failsThenSucceeds(errors.New("first"), errors.New("second"), last)

		Expect(withRetries(fastRetryPolicy, m, testJob, task)(context.Background())).To(MatchError(last))

		expectMetrics(`
# HELP requester_job_runs_total Scheduled job attempts, by job and result. A retried run counts once per attempt.
# TYPE requester_job_runs_total counter
requester_job_runs_total{job="test-job",result="failure"} 3
# HELP requester_job_retries_exhausted_total Scheduled job runs where every attempt failed, by job.
# TYPE requester_job_retries_exhausted_total counter
requester_job_retries_exhausted_total{job="test-job"} 1
`)
	})

	It("stops retrying without counting exhaustion when the context is canceled", func() {
		ctx, cancel := context.WithCancel(context.Background())
		task := func(context.Context) error {
			cancel()
			return errors.New("interrupted")
		}

		Expect(withRetries(fastRetryPolicy, m, testJob, task)(ctx)).To(MatchError(context.Canceled))

		expectMetrics(`
# HELP requester_job_runs_total Scheduled job attempts, by job and result. A retried run counts once per attempt.
# TYPE requester_job_runs_total counter
requester_job_runs_total{job="test-job",result="failure"} 1
`)
	})
})
