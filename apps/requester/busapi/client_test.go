package busapi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type capturedRequest struct {
	method string
	path   string
	query  string
	header http.Header
	body   string
}

func newTestClient(response string) (*Client, *capturedRequest) {
	var captured capturedRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		captured = capturedRequest{
			method: r.Method,
			path:   r.URL.Path,
			query:  r.URL.RawQuery,
			header: r.Header.Clone(),
			body:   string(body),
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, response)
	}))
	DeferCleanup(server.Close)

	auth := StaticAuth{"Cookie": "session=abc"}
	return NewClient(ClientConfig{Auth: auth, BaseURL: server.URL, HTTPClient: server.Client()}), &captured
}

var _ = Describe("Client", Label("unit"), func() {
	ctx := context.Background()

	It("sends auth headers", func() {
		client, captured := newTestClient(`{"routes":[],"serviceInterruptions":[]}`)

		_, err := client.GetBaseData(ctx)
		Expect(err).NotTo(HaveOccurred())

		Expect(captured.header.Get("Cookie")).To(Equal("session=abc"))
		Expect(captured.method).To(Equal(http.MethodPost))
		Expect(captured.path).To(Equal(pathBaseData))
	})

	It("form encodes route keys for pattern paths", func() {
		client, captured := newTestClient(`[]`)

		_, err := client.GetPatternPaths(ctx, []string{"abc", "d/e"})
		Expect(err).NotTo(HaveOccurred())

		Expect(captured.body).To(Equal("routeKeys%5B%5D=abc&routeKeys%5B%5D=d%2Fe"))
		Expect(captured.header.Get("Content-Type")).To(Equal(contentTypeForm))
	})

	It("indexes each direction for next departure times", func() {
		client, captured := newTestClient(`{"amenities":[],"routeDirectionTimes":[],"stopCode":"100"}`)

		routeDirectionPairs := []RouteDirectionPair{
			{RouteKey: "route1", DirectionKey: "dir1"},
			{RouteKey: "route1", DirectionKey: "dir2"},
		}

		_, err := client.GetNextDepartureTimes(ctx, routeDirectionPairs, "100")
		Expect(err).NotTo(HaveOccurred())

		Expect(captured.body).To(Equal("routeDirectionKeys%5B0%5D%5BrouteKey%5D=route1" +
			"&routeDirectionKeys%5B0%5D%5BdirectionKey%5D=dir1&stopCode=100" +
			"&routeDirectionKeys%5B1%5D%5BrouteKey%5D=route1" +
			"&routeDirectionKeys%5B1%5D%5BdirectionKey%5D=dir2&stopCode=100"))
	})

	It("fills defaults for nearby routes", func() {
		client, captured := newTestClient(`{"routeResults":[]}`)

		_, err := client.GetNearbyRoutes(ctx, NearbyRoutesQuery{})
		Expect(err).NotTo(HaveOccurred())

		var payload nearbyRoutesPayload
		Expect(json.Unmarshal([]byte(captured.body), &payload)).To(Succeed())

		Expect(payload).To(Equal(nearbyRoutesPayload{
			Latitude:        defaultLatitude,
			Longitude:       defaultLongitude,
			MinRadius:       defaultMinRadius,
			MaxRadius:       defaultMaxRadius,
			FavouriteRoutes: []string{},
		}))
	})

	It("formats the date for stop schedules", func() {
		client, captured := newTestClient(`{"amenities":[],"date":"","routeStopSchedules":[]}`)

		date := time.Date(2026, time.March, 4, 13, 45, 0, 0, time.UTC)
		_, err := client.GetStopSchedules(ctx, "1234", date)
		Expect(err).NotTo(HaveOccurred())

		Expect(captured.body).To(Equal(`{"stopCode":"1234","date":"2026-03-04"}`))
	})

	It("searches bus stops with a query parameter", func() {
		client, captured := newTestClient(`[]`)

		_, err := client.FindBusStops(ctx, "memorial student center")
		Expect(err).NotTo(HaveOccurred())

		Expect(captured.method).To(Equal(http.MethodGet))
		Expect(captured.query).To(Equal("searchTerm=memorial+student+center"))
		Expect(captured.header.Get("Accept")).To(Equal(acceptAjax))
	})

	It("omits unset trip plan times and sends a null lang", func() {
		client, captured := newTestClient(`{"resultCount":0}`)

		departAt := time.Unix(1772000000, 0)
		query := TripPlanQuery{
			Origin:      Endpoint{Title: "MSC"},
			Destination: Endpoint{Title: "Kyle Field"},
			DepartAt:    &departAt,
		}
		_, err := client.GetTripPlan(ctx, query)
		Expect(err).NotTo(HaveOccurred())

		var body map[string]any
		Expect(json.Unmarshal([]byte(captured.body), &body)).To(Succeed())

		Expect(body).NotTo(HaveKey("arriveTime"))
		Expect(body).To(HaveKeyWithValue("departTime", "1772000000"))
		Expect(body).To(HaveKeyWithValue("lang", BeNil()))
	})

	It("carries the response body on a status error", func() {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			io.WriteString(w, "session expired")
		}))
		DeferCleanup(server.Close)

		client := NewClient(ClientConfig{Auth: StaticAuth{}, BaseURL: server.URL, HTTPClient: server.Client()})

		_, err := client.GetActiveRoutes(ctx)

		var statusErr *StatusError
		Expect(err).To(BeAssignableToTypeOf(statusErr))
		statusErr = err.(*StatusError)
		Expect(statusErr.StatusCode).To(Equal(http.StatusUnauthorized))
		Expect(statusErr.Body).To(Equal("session expired"))
	})
})

var _ = Describe("TripPlanQuery.payload", Label("unit"), func() {
	It("sends empty strings for unset endpoint fields", func() {
		stopCode := "1234"
		latitude := 30.61

		payload := TripPlanQuery{
			Origin: Endpoint{
				Title:    "MSC",
				Subtitle: "College Station",
				StopCode: &stopCode,
				Latitude: &latitude,
			},
			Destination: Endpoint{Title: "Kyle Field"},
		}.payload()

		Expect(payload.OriginLatitude).To(Equal(latitude))
		Expect(payload.OriginStopCode).To(Equal(stopCode))
		Expect(payload.OriginLongitude).To(Equal(""))
		Expect(payload.DestinationPlaceID).To(Equal(""))
	})
})
