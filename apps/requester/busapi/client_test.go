package busapi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type capturedRequest struct {
	method string
	path   string
	query  string
	header http.Header
	body   string
}

func newTestClient(t *testing.T, response string) (*Client, *capturedRequest) {
	t.Helper()

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
	t.Cleanup(server.Close)

	auth := StaticAuth{"Cookie": "session=abc"}
	return NewClient(ClientConfig{Auth: auth, BaseURL: server.URL, HTTPClient: server.Client()}), &captured
}

func TestAuthHeadersAreSent(t *testing.T) {
	client, captured := newTestClient(t, `{"routes":[],"serviceInterruptions":[]}`)

	if _, err := client.GetBaseData(context.Background()); err != nil {
		t.Fatalf("GetBaseData: %v", err)
	}

	if got := captured.header.Get("Cookie"); got != "session=abc" {
		t.Errorf("Cookie header = %q, want %q", got, "session=abc")
	}
	if captured.method != http.MethodPost {
		t.Errorf("method = %q, want POST", captured.method)
	}
	if captured.path != pathBaseData {
		t.Errorf("path = %q, want %q", captured.path, pathBaseData)
	}
}

func TestGetPatternPathsEncodesRouteKeys(t *testing.T) {
	client, captured := newTestClient(t, `[]`)

	if _, err := client.GetPatternPaths(context.Background(), []string{"abc", "d/e"}); err != nil {
		t.Fatalf("GetPatternPaths: %v", err)
	}

	want := "routeKeys%5B%5D=abc&routeKeys%5B%5D=d%2Fe"
	if captured.body != want {
		t.Errorf("body = %q, want %q", captured.body, want)
	}
	if got := captured.header.Get("Content-Type"); got != contentTypeForm {
		t.Errorf("Content-Type = %q, want %q", got, contentTypeForm)
	}
}

func TestGetNextDepartureTimesIndexesEachDirection(t *testing.T) {
	client, captured := newTestClient(t, `{"amenities":[],"routeDirectionTimes":[],"stopCode":"100"}`)

	routeDirectionPairs := []RouteDirectionPair{
		{RouteKey: "route1", DirectionKey: "dir1"},
		{RouteKey: "route1", DirectionKey: "dir2"},
	}

	_, err := client.GetNextDepartureTimes(context.Background(), routeDirectionPairs, "100")
	if err != nil {
		t.Fatalf("GetNextDepartureTimes: %v", err)
	}

	want := "routeDirectionKeys%5B0%5D%5BrouteKey%5D=route1" +
		"&routeDirectionKeys%5B0%5D%5BdirectionKey%5D=dir1&stopCode=100" +
		"&routeDirectionKeys%5B1%5D%5BrouteKey%5D=route1" +
		"&routeDirectionKeys%5B1%5D%5BdirectionKey%5D=dir2&stopCode=100"
	if captured.body != want {
		t.Errorf("body = %q, want %q", captured.body, want)
	}
}

func TestGetNearbyRoutesFillsDefaults(t *testing.T) {
	client, captured := newTestClient(t, `{"routeResults":[]}`)

	if _, err := client.GetNearbyRoutes(context.Background(), NearbyRoutesQuery{}); err != nil {
		t.Fatalf("GetNearbyRoutes: %v", err)
	}

	var payload nearbyRoutesPayload
	if err := json.Unmarshal([]byte(captured.body), &payload); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}

	want := nearbyRoutesPayload{
		Latitude:        defaultLatitude,
		Longitude:       defaultLongitude,
		MinRadius:       defaultMinRadius,
		MaxRadius:       defaultMaxRadius,
		FavouriteRoutes: []string{},
	}
	if payload.Latitude != want.Latitude || payload.Longitude != want.Longitude {
		t.Errorf("position = (%v, %v), want (%v, %v)", payload.Latitude, payload.Longitude, want.Latitude, want.Longitude)
	}
	if payload.MinRadius != want.MinRadius || payload.MaxRadius != want.MaxRadius {
		t.Errorf("radii = (%v, %v), want (%v, %v)", payload.MinRadius, payload.MaxRadius, want.MinRadius, want.MaxRadius)
	}
	if payload.FavouriteRoutes == nil {
		t.Error("favouriteRoutes was null, want an empty array")
	}
}

func TestGetStopSchedulesFormatsDate(t *testing.T) {
	client, captured := newTestClient(t, `{"amenities":[],"date":"","routeStopSchedules":[]}`)

	date := time.Date(2026, time.March, 4, 13, 45, 0, 0, time.UTC)
	if _, err := client.GetStopSchedules(context.Background(), "1234", date); err != nil {
		t.Fatalf("GetStopSchedules: %v", err)
	}

	want := `{"stopCode":"1234","date":"2026-03-04"}`
	if captured.body != want {
		t.Errorf("body = %q, want %q", captured.body, want)
	}
}

func TestFindBusStopsUsesQueryParameter(t *testing.T) {
	client, captured := newTestClient(t, `[]`)

	if _, err := client.FindBusStops(context.Background(), "memorial student center"); err != nil {
		t.Fatalf("FindBusStops: %v", err)
	}

	if captured.method != http.MethodGet {
		t.Errorf("method = %q, want GET", captured.method)
	}
	if want := "searchTerm=memorial+student+center"; captured.query != want {
		t.Errorf("query = %q, want %q", captured.query, want)
	}
	if got := captured.header.Get("Accept"); got != acceptAjax {
		t.Errorf("Accept = %q, want %q", got, acceptAjax)
	}
}

func TestTripPlanPayloadSendsEmptyStringsForUnsetEndpointFields(t *testing.T) {
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

	if payload.OriginLatitude != latitude {
		t.Errorf("originLatitude = %v, want %v", payload.OriginLatitude, latitude)
	}
	if payload.OriginStopCode != stopCode {
		t.Errorf("originStopCode = %v, want %v", payload.OriginStopCode, stopCode)
	}
	if payload.OriginLongitude != "" {
		t.Errorf("originLongitude = %v, want an empty string", payload.OriginLongitude)
	}
	if payload.DestinationPlaceID != "" {
		t.Errorf("destinationPlaceId = %v, want an empty string", payload.DestinationPlaceID)
	}
}

func TestTripPlanOmitsUnsetTimesAndSendsNullLang(t *testing.T) {
	client, captured := newTestClient(t, `{"resultCount":0}`)

	departAt := time.Unix(1772000000, 0)
	query := TripPlanQuery{
		Origin:      Endpoint{Title: "MSC"},
		Destination: Endpoint{Title: "Kyle Field"},
		DepartAt:    &departAt,
	}
	if _, err := client.GetTripPlan(context.Background(), query); err != nil {
		t.Fatalf("GetTripPlan: %v", err)
	}

	var body map[string]any
	if err := json.Unmarshal([]byte(captured.body), &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}

	if _, ok := body["arriveTime"]; ok {
		t.Error("arriveTime was sent, want it omitted")
	}
	if got := body["departTime"]; got != "1772000000" {
		t.Errorf("departTime = %v, want %q", got, "1772000000")
	}
	lang, ok := body["lang"]
	if !ok || lang != nil {
		t.Errorf("lang = %v (present: %v), want an explicit null", lang, ok)
	}
}

func TestStatusErrorCarriesResponseBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		io.WriteString(w, "session expired")
	}))
	t.Cleanup(server.Close)

	client := NewClient(ClientConfig{Auth: StaticAuth{}, BaseURL: server.URL, HTTPClient: server.Client()})

	_, err := client.GetActiveRoutes(context.Background())
	statusErr, ok := err.(*StatusError)
	if !ok {
		t.Fatalf("error = %v, want *StatusError", err)
	}
	if statusErr.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", statusErr.StatusCode, http.StatusUnauthorized)
	}
	if statusErr.Body != "session expired" {
		t.Errorf("body = %q, want %q", statusErr.Body, "session expired")
	}
}
