package metrics_test

import (
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/gorilla/websocket"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/MaroonRides/api/apps/gateway/metrics"
)

var _ = Describe("HTTPMetrics", Label("unit"), func() {
	var (
		registry *prometheus.Registry
		handler  http.Handler
	)

	BeforeEach(func() {
		registry = prometheus.NewRegistry()
		mux := http.NewServeMux()
		mux.HandleFunc("GET /api/routes/{id}", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusTeapot)
		})
		mux.HandleFunc("GET /api/ws", func(w http.ResponseWriter, r *http.Request) {
			conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
			if err == nil {
				conn.Close()
			}
		})
		handler = metrics.NewHTTPMetrics(registry).Middleware(mux)
	})

	serve := func(path string) {
		handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, path, nil))
	}

	It("labels requests with the matched route instead of the path", func() {
		serve("/api/routes/a")
		serve("/api/routes/b")

		Expect(testutil.GatherAndCompare(registry, strings.NewReader(`
# HELP gateway_http_requests_total HTTP requests served, by route and status.
# TYPE gateway_http_requests_total counter
gateway_http_requests_total{route="GET /api/routes/{id}",status="418"} 2
`), "gateway_http_requests_total")).To(Succeed())
	})

	It("groups requests that match no route", func() {
		serve("/nope")

		Expect(testutil.GatherAndCompare(registry, strings.NewReader(`
# HELP gateway_http_requests_total HTTP requests served, by route and status.
# TYPE gateway_http_requests_total counter
gateway_http_requests_total{route="unmatched",status="404"} 1
`), "gateway_http_requests_total")).To(Succeed())
	})

	It("leaves websocket upgrades out", func() {
		server := httptest.NewServer(handler)
		DeferCleanup(server.Close)

		conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"/api/ws", nil)
		Expect(err).NotTo(HaveOccurred())
		conn.Close()

		Expect(testutil.CollectAndCount(registry, "gateway_http_requests_total")).To(BeZero())
	})
})
