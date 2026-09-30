package controllers

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
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

func readLines(body io.Reader) []wireLine {
	var lines []wireLine
	scanner := bufio.NewScanner(body)
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

	postWith := func(body string, header http.Header) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, streamPath, strings.NewReader(body))
		req.Header = header
		req.Header.Set("Content-Type", "application/json")
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		return res
	}
	post := func(body string) *httptest.ResponseRecorder {
		return postWith(body, http.Header{})
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

		res := post(`{"protocol":1,"types":["RoutesV1","StopsV1"],"acks":[]}`)

		Expect(res.Code).To(Equal(http.StatusOK), res.Body.String())
		Expect(res.Header().Get("Content-Type")).To(Equal(controllers.JSONLinesContentType))

		lines := readLines(res.Body)
		Expect(lines).To(HaveLen(3))
		Expect(lines[0].Type).To(Equal(dtos.SyncEntityTypes.RouteV1))
		Expect(lines[0].Ack).To(HavePrefix("RouteV1|"))
		Expect(lines[0].Data).To(HaveKeyWithValue("id", network.Route.ID.String()))
		Expect(lines[0].Data).To(HaveKeyWithValue("shortName", "01"))
		Expect(lines[1].Type).To(Equal(dtos.SyncEntityTypes.StopV1))
		Expect(lines[2].Type).To(Equal(dtos.SyncEntityTypes.SyncCompleteV1))
		Expect(lines[2].Data).To(BeEmpty())
	})

	It("streams timetables for the scoped routes", func() {
		kept := test.CreateNetwork(bundb, "01")
		test.CreateNetwork(bundb, "02")
		time.Sleep(2 * repositories.NowIDLag)

		res := post(fmt.Sprintf(`{"protocol":1,"types":["TimetablesV1"],"scopes":[{"type":"OfflineRoutesV1","ids":[%q]}],"acks":[]}`, kept.Route.ID))

		Expect(res.Code).To(Equal(http.StatusOK), res.Body.String())
		lines := readLines(res.Body)
		Expect(lines).To(HaveLen(2))
		Expect(lines[0].Ack).To(HavePrefix("TimetableV1:" + kept.Route.ID.String() + "|"))
		Expect(lines[0].Data).To(HaveKeyWithValue("id", kept.Timetable.ID.String()))
	})

	It("rejects an unknown scope type", func() {
		res := post(`{"protocol":1,"types":["TimetablesV1"],"scopes":[{"type":"Stop","ids":[]}]}`)

		Expect(res.Code).To(Equal(http.StatusBadRequest), res.Body.String())
	})

	Describe("compression", func() {
		const routesAndStops = `{"protocol":1,"types":["RoutesV1","StopsV1"],"acks":[]}`
		const networks = 20

		// gzhttp leaves small bodies alone, so the stream needs enough rows to be worth compressing.
		BeforeEach(func() {
			for i := range networks {
				test.CreateNetwork(bundb, fmt.Sprintf("%02d", i))
			}
			time.Sleep(2 * repositories.NowIDLag)
		})

		It("gzips the lines for a client that accepts gzip", func() {
			res := postWith(routesAndStops, http.Header{"Accept-Encoding": {"gzip"}})

			Expect(res.Code).To(Equal(http.StatusOK), res.Body.String())
			Expect(res.Header().Get("Content-Encoding")).To(Equal("gzip"))
			zr, err := gzip.NewReader(res.Body)
			Expect(err).NotTo(HaveOccurred())
			Expect(readLines(zr)).To(HaveLen(2*networks + 1))
		})

		It("sends plain lines to a client that does not ask for gzip", func() {
			res := post(routesAndStops)

			Expect(res.Code).To(Equal(http.StatusOK), res.Body.String())
			Expect(res.Header().Get("Content-Encoding")).To(BeEmpty())
			Expect(readLines(res.Body)).To(HaveLen(2*networks + 1))
		})
	})

	It("answers with a server error when the database fails before any line", func() {
		Expect(bundb.Close()).To(Succeed())

		res := post(`{"protocol":1,"types":["RoutesV1"]}`)

		Expect(res.Code).To(Equal(http.StatusInternalServerError), res.Body.String())
		Expect(res.Header().Get("Content-Type")).NotTo(Equal(controllers.JSONLinesContentType))
	})
})
