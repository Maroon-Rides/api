package metrics

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/fx"
)

const (
	namespace = "requester"

	labelJob      = "job"
	labelResult   = "result"
	labelUpstream = "upstream"
	labelPath     = "path"
	labelStatus   = "status"

	resultSuccess = "success"
	resultFailure = "failure"

	// Stands in for the status code when no response came back.
	statusTransportError = "error"
)

// Jobs and upstream calls run from tens of milliseconds to about a minute.
var durationBuckets = prometheus.ExponentialBuckets(0.05, 2, 12)

var Module = fx.Provide(NewJobMetrics, NewUpstreamMetrics)

type JobMetrics struct {
	runs     *prometheus.CounterVec
	duration *prometheus.HistogramVec
}

func NewJobMetrics(registry prometheus.Registerer) *JobMetrics {
	m := &JobMetrics{
		runs: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "job_runs_total",
			Help:      "Scheduled job runs, by job and result.",
		}, []string{labelJob, labelResult}),
		duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "job_duration_seconds",
			Help:      "Time a scheduled job run took, by job.",
			Buckets:   durationBuckets,
		}, []string{labelJob}),
	}
	registry.MustRegister(m.runs, m.duration)
	return m
}

func (m *JobMetrics) Instrument(name string, task func(context.Context) error) func(context.Context) error {
	return func(ctx context.Context) error {
		start := time.Now()
		err := task(ctx)
		m.duration.WithLabelValues(name).Observe(time.Since(start).Seconds())

		result := resultSuccess
		if err != nil {
			result = resultFailure
		}
		m.runs.WithLabelValues(name, result).Inc()
		return err
	}
}

type UpstreamMetrics struct {
	requests *prometheus.CounterVec
	duration *prometheus.HistogramVec
}

func NewUpstreamMetrics(registry prometheus.Registerer) *UpstreamMetrics {
	m := &UpstreamMetrics{
		requests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "upstream_requests_total",
			Help:      "Requests to upstream APIs, by upstream, path and status.",
		}, []string{labelUpstream, labelPath, labelStatus}),
		duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "upstream_request_duration_seconds",
			Help:      "Time until an upstream API responded, by upstream and path.",
			Buckets:   durationBuckets,
		}, []string{labelUpstream, labelPath}),
	}
	registry.MustRegister(m.requests, m.duration)
	return m
}

func (m *UpstreamMetrics) Transport(upstream string) func(http.RoundTripper) http.RoundTripper {
	return func(next http.RoundTripper) http.RoundTripper {
		return roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			start := time.Now()
			res, err := next.RoundTrip(req)
			m.duration.WithLabelValues(upstream, req.URL.Path).Observe(time.Since(start).Seconds())

			status := statusTransportError
			if err == nil {
				status = strconv.Itoa(res.StatusCode)
			}
			m.requests.WithLabelValues(upstream, req.URL.Path, status).Inc()
			return res, err
		})
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
