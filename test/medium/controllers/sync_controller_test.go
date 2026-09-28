package controllers

import (
	"bufio"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"github.com/go-fuego/fuego"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/uptrace/bun"

	"github.com/MaroonRides/api/apps/gateway/controllers"
	"github.com/MaroonRides/api/apps/gateway/dtos"
	"github.com/MaroonRides/api/apps/gateway/repositories"
	"github.com/MaroonRides/api/apps/gateway/services"
	"github.com/MaroonRides/api/test"
)

const streamPath = "/api/sync/stream"

type wireLine struct {
	Type dtos.SyncEntityType `json:"type"`
	Ack  string              `json:"ack"`
	Data map[string]any      `json:"data"`
}

func readLines(res *httptest.ResponseRecorder) []wireLine {
	var lines []wireLine
	scanner := bufio.NewScanner(res.Body)
	for scanner.Scan() {
		var line wireLine
		ExpectWithOffset(1, json.Unmarshal(scanner.Bytes(), &line)).To(Succeed(), "line %q is not JSON", scanner.Text())
		lines = append(lines, line)
	}
	return lines
}

var _ = Describe("SyncController", Label("medium", "controller"), func() {
	var bundb *bun.DB
	var handler http.Handler

	post := func(body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, streamPath, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		return res
	}

	BeforeEach(func() {
		bundb = test.Configure()
		server := fuego.NewServer()
		controller := controllers.NewSyncController(services.NewSyncService(repositories.NewSyncRepository(bundb)))
		controller.Register(fuego.Group(server, "/api"))
		handler = server.Mux
	})

	It("streams JSON lines", func() {
		network := test.CreateNetwork(bundb, "01")
		time.Sleep(2 * repositories.NowIDLag)

		res := post(`{"types":["RoutesV1","StopsV1"],"acks":[]}`)

		Expect(res.Code).To(Equal(http.StatusOK), res.Body.String())
		Expect(res.Header().Get("Content-Type")).To(Equal(controllers.JSONLinesContentType))

		lines := readLines(res)
		Expect(lines).To(HaveLen(3))
		Expect(lines[0].Type).To(Equal(dtos.SyncEntityTypes.RouteV1))
		Expect(lines[0].Ack).To(HavePrefix("RouteV1|"))
		Expect(lines[0].Data).To(HaveKeyWithValue("id", network.Route.ID.String()))
		Expect(lines[0].Data).To(HaveKeyWithValue("shortName", "01"))
		Expect(lines[1].Type).To(Equal(dtos.SyncEntityTypes.StopV1))
		Expect(lines[2].Type).To(Equal(dtos.SyncEntityTypes.SyncCompleteV1))
		Expect(lines[2].Data).To(BeEmpty())
	})

	It("answers with a server error when the database fails before any line", func() {
		Expect(bundb.Close()).To(Succeed())

		res := post(`{"types":["RoutesV1"]}`)

		Expect(res.Code).To(Equal(http.StatusInternalServerError), res.Body.String())
		Expect(res.Header().Get("Content-Type")).NotTo(Equal(controllers.JSONLinesContentType))
	})
})
