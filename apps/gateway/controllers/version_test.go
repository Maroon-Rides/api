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

const appSessionsMetric = "gateway_app_sessions_total"

var _ = Describe("VersionController", Label("unit"), func() {
	var (
		handler  http.Handler
		registry *prometheus.Registry
	)

	BeforeEach(func() {
		registry = prometheus.NewRegistry()
		server := fuego.NewServer()
		controllers.NewVersionController(services.NewSessionService(metrics.NewSessionMetrics(registry))).Register(fuego.Group(server, "/api"))
		handler = server.Mux
	})

	checkVersion := func(userAgent string) {
		req := httptest.NewRequest(http.MethodGet, "/api/version/supported", nil)
		req.Header.Set("User-Agent", userAgent)
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		ExpectWithOffset(1, res.Code).To(Equal(http.StatusOK))
	}

	It("counts a session by platform and app version", func() {
		checkVersion("Maroon Rides App (iOS) v3.0.0")
		checkVersion("Maroon Rides App (iOS) v3.0.0")
		checkVersion("Maroon Rides App (Android) v3.0.1")

		Expect(testutil.GatherAndCompare(registry, strings.NewReader(`
# HELP gateway_app_sessions_total App launches, counted by the version check each launch makes, by platform and app version.
# TYPE gateway_app_sessions_total counter
gateway_app_sessions_total{platform="android",version="3.0.1"} 1
gateway_app_sessions_total{platform="ios",version="3.0.0"} 2
`), appSessionsMetric)).To(Succeed())
	})

	It("leaves out callers that are not the app", func() {
		checkVersion("kube-probe/1.34")
		checkVersion("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)")

		Expect(testutil.CollectAndCount(registry, appSessionsMetric)).To(BeZero())
	})

	It("folds versions past the cap into one label", func() {
		for patch := range 60 {
			checkVersion(fmt.Sprintf("Maroon Rides App (iOS) v3.0.%d", patch))
		}

		Expect(testutil.CollectAndCount(registry, appSessionsMetric)).To(Equal(51))
		checkVersion("Maroon Rides App (iOS) v3.0.0")
		Expect(testutil.CollectAndCount(registry, appSessionsMetric)).To(Equal(51))
	})
})
