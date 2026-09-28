package controllers_test

import (
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/go-fuego/fuego"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/MaroonRides/api/apps/gateway/controllers"
	"github.com/MaroonRides/api/apps/gateway/services"
)

// A nil repository proves validation happens before the database is touched.
var _ = Describe("SyncController", Label("unit"), func() {
	var handler http.Handler

	BeforeEach(func() {
		server := fuego.NewServer()
		controllers.NewSyncController(services.NewSyncService(nil)).Register(fuego.Group(server, "/api"))
		handler = server.Mux
	})

	DescribeTable("rejects bad requests before streaming",
		func(body string) {
			req := httptest.NewRequest(http.MethodPost, "/api/sync/stream", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			res := httptest.NewRecorder()

			handler.ServeHTTP(res, req)

			Expect(res.Code).To(Equal(http.StatusBadRequest), res.Body.String())
			Expect(res.Header().Get("Content-Type")).NotTo(ContainSubstring(controllers.JSONLinesContentType))
		},
		Entry("an unknown type", `{"types":["TripsV1"]}`),
		Entry("no types", `{"types":[]}`),
		Entry("a bad ack", `{"types":["RoutesV1"],"acks":["RouteV1|nope"]}`),
		Entry("a malformed body", `{"types":`),
	)
})
