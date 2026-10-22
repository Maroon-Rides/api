package controllers_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/go-fuego/fuego"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/MaroonRides/api/apps/gateway/controllers"
	"github.com/MaroonRides/api/apps/gateway/metrics"
	"github.com/MaroonRides/api/apps/gateway/services"
)

const (
	syncFailuresMetric = "gateway_sync_failures_total"
	reportEventsMetric = "gateway_report_events_total"
)

var _ = Describe("ReportController", Label("unit"), func() {
	var (
		handler  http.Handler
		registry *prometheus.Registry
	)

	BeforeEach(func() {
		registry = prometheus.NewRegistry()
		server := fuego.NewServer()
		svc := services.NewReportService(metrics.NewSyncMetrics(registry), metrics.NewReportMetrics(registry))
		controllers.NewReportController(svc).Register(fuego.Group(server, "/api"))
		handler = server.Mux
	})

	report := func(events ...string) *httptest.ResponseRecorder {
		body := fmt.Sprintf(`{"events":[%s]}`, strings.Join(events, ","))
		req := httptest.NewRequest(http.MethodPost, "/api/reports", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		return res
	}

	expectMetrics := func(expected string, names ...string) {
		ExpectWithOffset(1, testutil.GatherAndCompare(registry, strings.NewReader(expected), names...)).To(Succeed())
	}

	It("counts a failure from an empty database apart from one with checkpoints", func() {
		res := report(
			`{"type":"SyncFailedInvalidLineV1","payload":{"protocol":1,"acks":[],"message":"bad line"}}`,
			`{"type":"SyncFailedApplyV1","payload":{"protocol":1,"acks":["RouteV1|x"]}}`,
			`{"type":"SyncFailedHTTPStatusV1","payload":{"protocol":1,"acks":["nope"],"status":400}}`,
		)

		Expect(res.Code).To(Equal(http.StatusNoContent), res.Body.String())
		expectMetrics(`
# HELP gateway_sync_failures_total Failed syncs reported by clients, by failure code and what the sync started from.
# TYPE gateway_sync_failures_total counter
gateway_sync_failures_total{code="apply",start="checkpoint"} 1
gateway_sync_failures_total{code="apply",start="empty"} 0
gateway_sync_failures_total{code="http_status",start="checkpoint"} 1
gateway_sync_failures_total{code="http_status",start="empty"} 0
gateway_sync_failures_total{code="invalid_line",start="checkpoint"} 0
gateway_sync_failures_total{code="invalid_line",start="empty"} 1
`, syncFailuresMetric)
	})

	It("drops events it cannot read and keeps the rest", func() {
		res := report(
			`{"type":"TripPlannedV9","payload":{}}`,
			`{"type":"SyncFailedApplyV1","payload":{"protocol":9,"acks":[]}}`,
			`{"type":"SyncFailedApplyV1","payload":"nope"}`,
			fmt.Sprintf(`{"type":"SyncFailedApplyV1","payload":{"protocol":1,"acks":["%s"]}}`, strings.Repeat(`a","`, 64)+"a"),
			`{"type":"SyncFailedApplyV1","payload":{"protocol":1,"acks":[]}}`,
		)

		Expect(res.Code).To(Equal(http.StatusNoContent), res.Body.String())
		expectMetrics(`
# HELP gateway_report_events_total Client report events received, by type and outcome.
# TYPE gateway_report_events_total counter
gateway_report_events_total{outcome="dropped",type="SyncFailedApplyV1"} 3
gateway_report_events_total{outcome="dropped",type="unknown"} 1
gateway_report_events_total{outcome="recorded",type="SyncFailedApplyV1"} 1
`, reportEventsMetric)
	})

	DescribeTable("rejects bad requests",
		func(body string) {
			req := httptest.NewRequest(http.MethodPost, "/api/reports", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			res := httptest.NewRecorder()
			handler.ServeHTTP(res, req)

			Expect(res.Code).To(Equal(http.StatusBadRequest), res.Body.String())
			Expect(testutil.CollectAndCount(registry, reportEventsMetric)).To(BeZero())
		},
		Entry("too many events", fmt.Sprintf(`{"events":[%s]}`, strings.TrimSuffix(strings.Repeat(`{"type":"x","payload":{}},`, 51), ","))),
		Entry("a malformed body", `{"events":`),
		Entry("a body past the size limit", fmt.Sprintf(`{"events":[],"pad":"%s"}`, strings.Repeat("a", 256<<10))),
	)
})
