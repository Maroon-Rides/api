package metrics_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/MaroonRides/api/apps/requester/metrics"
)

var _ = Describe("JobMetrics", Label("unit"), func() {
	It("counts runs by result and passes the error through", func() {
		registry := prometheus.NewRegistry()
		m := metrics.NewJobMetrics(registry)
		failure := errors.New("boom")

		Expect(m.Instrument("ok", func(context.Context) error { return nil })(context.Background())).To(Succeed())
		Expect(m.Instrument("bad", func(context.Context) error { return failure })(context.Background())).To(MatchError(failure))

		Expect(testutil.GatherAndCompare(registry, strings.NewReader(`
# HELP requester_job_runs_total Scheduled job runs, by job and result.
# TYPE requester_job_runs_total counter
requester_job_runs_total{job="bad",result="failure"} 1
requester_job_runs_total{job="ok",result="success"} 1
`), "requester_job_runs_total")).To(Succeed())
	})
})

var _ = Describe("UpstreamMetrics", Label("unit"), func() {
	var (
		registry *prometheus.Registry
		client   *http.Client
	)

	BeforeEach(func() {
		registry = prometheus.NewRegistry()
		client = &http.Client{Transport: metrics.NewUpstreamMetrics(registry).Transport("busapi")(http.DefaultTransport)}
	})

	It("labels requests by path and status, leaving out the query", func() {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusAccepted)
		}))
		DeferCleanup(server.Close)

		res, err := client.Get(server.URL + "/RouteMap/GetVehicles?route=1")
		Expect(err).NotTo(HaveOccurred())
		res.Body.Close()

		Expect(testutil.GatherAndCompare(registry, strings.NewReader(`
# HELP requester_upstream_requests_total Requests to upstream APIs, by upstream, path and status.
# TYPE requester_upstream_requests_total counter
requester_upstream_requests_total{path="/RouteMap/GetVehicles",status="202",upstream="busapi"} 1
`), "requester_upstream_requests_total")).To(Succeed())
	})

	It("records a request that got no response", func() {
		server := httptest.NewServer(http.NotFoundHandler())
		server.Close()

		_, err := client.Get(server.URL + "/Home/GetActiveRoutes")
		Expect(err).To(HaveOccurred())

		Expect(testutil.GatherAndCompare(registry, strings.NewReader(`
# HELP requester_upstream_requests_total Requests to upstream APIs, by upstream, path and status.
# TYPE requester_upstream_requests_total counter
requester_upstream_requests_total{path="/Home/GetActiveRoutes",status="error",upstream="busapi"} 1
`), "requester_upstream_requests_total")).To(Succeed())
	})
})
