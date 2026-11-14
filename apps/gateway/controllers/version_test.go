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
	appSessionsMetric = "gateway_app_sessions_total"
	appSessionsHeader = `
# HELP gateway_app_sessions_total App launches, counted by the version check each launch makes, by platform and app version.
# TYPE gateway_app_sessions_total counter
`
	browserUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)"
)

type appRequest struct {
	platform  string
	version   string
	userAgent string
}

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

	checkVersion := func(app appRequest) {
		req := httptest.NewRequest(http.MethodGet, "/api/version/supported", nil)
		req.Header.Set("User-Agent", app.userAgent)
		if app.platform != "" {
			req.Header.Set("X-App-Platform", app.platform)
		}
		if app.version != "" {
			req.Header.Set("X-App-Version", app.version)
		}
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		ExpectWithOffset(1, res.Code).To(Equal(http.StatusOK))
	}

	expectSessions := func(series string) {
		ExpectWithOffset(1, testutil.GatherAndCompare(registry, strings.NewReader(appSessionsHeader+series), appSessionsMetric)).To(Succeed())
	}

	It("counts a session by the platform and version headers", func() {
		checkVersion(appRequest{platform: "ios", version: "3.0.2"})
		checkVersion(appRequest{platform: "ios", version: "3.0.2"})
		checkVersion(appRequest{platform: "android", version: "3.0.2"})
		checkVersion(appRequest{platform: "web", version: "3.0.2", userAgent: browserUserAgent})

		expectSessions(`
gateway_app_sessions_total{platform="android",version="3.0.2"} 1
gateway_app_sessions_total{platform="ios",version="3.0.2"} 2
gateway_app_sessions_total{platform="web",version="3.0.2"} 1
`)
	})

	It("prefers the headers over the user agent", func() {
		checkVersion(appRequest{platform: "android", version: "3.1.0", userAgent: "Maroon Rides App (iOS) v3.0.0"})

		expectSessions(`
gateway_app_sessions_total{platform="android",version="3.1.0"} 1
`)
	})

	It("counts a build that only names itself in the user agent", func() {
		checkVersion(appRequest{userAgent: "Maroon Rides App (iOS) v3.0.0"})
		checkVersion(appRequest{userAgent: "Maroon Rides App (iOS) v3.0.0"})
		checkVersion(appRequest{userAgent: "Maroon Rides App (Android) v3.0.1"})

		expectSessions(`
gateway_app_sessions_total{platform="android",version="3.0.1"} 1
gateway_app_sessions_total{platform="ios",version="3.0.0"} 2
`)
	})

	DescribeTable("falls back to the user agent when the headers are not usable",
		func(platform, version string) {
			checkVersion(appRequest{platform: platform, version: version, userAgent: "Maroon Rides App (Android) v3.0.1"})

			expectSessions(`
gateway_app_sessions_total{platform="android",version="3.0.1"} 1
`)
		},
		Entry("platform only", "web", ""),
		Entry("version only", "", "3.0.2"),
		Entry("unknown platform", "electron", "3.0.2"),
		Entry("platform not lowercase", "iOS", "3.0.2"),
		Entry("version with a v prefix", "web", "v3.0.2"),
		Entry("version missing a part", "web", "3.0"),
		Entry("version with a prerelease", "web", "3.0.2-beta.1"),
		Entry("version that is not a number", "web", "latest"),
	)

	DescribeTable("leaves out callers that are not the app",
		func(app appRequest) {
			checkVersion(app)

			Expect(testutil.CollectAndCount(registry, appSessionsMetric)).To(BeZero())
		},
		Entry("kubelet probe", appRequest{userAgent: "kube-probe/1.34"}),
		Entry("browser", appRequest{userAgent: browserUserAgent}),
		Entry("browser with a bad platform header", appRequest{platform: "linux", version: "3.0.2", userAgent: browserUserAgent}),
		Entry("browser with a bad version header", appRequest{platform: "web", version: "3", userAgent: browserUserAgent}),
	)

	It("folds versions past the cap into one label", func() {
		for patch := range 60 {
			checkVersion(appRequest{platform: "web", version: fmt.Sprintf("3.0.%d", patch)})
		}

		Expect(testutil.CollectAndCount(registry, appSessionsMetric)).To(Equal(51))
		checkVersion(appRequest{userAgent: "Maroon Rides App (iOS) v3.0.0"})
		checkVersion(appRequest{platform: "web", version: "3.0.0"})
		Expect(testutil.CollectAndCount(registry, appSessionsMetric)).To(Equal(52))
		checkVersion(appRequest{platform: "web", version: "4.0.0"})
		Expect(testutil.CollectAndCount(registry, appSessionsMetric)).To(Equal(52))
	})
})
