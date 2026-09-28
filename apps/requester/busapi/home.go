package busapi

import (
	"context"
	"time"
)

const (
	pathActiveRoutes  = "/Home/GetActiveRoutes"
	pathNearbyRoutes  = "/Home/GetNearbyRoutes"
	pathNextStopTimes = "/Home/GetNextStopTimes"
	pathStopSchedules = "/Schedule/GetStopSchedules"
	pathStopEstimates = "/Schedule/GetStopEstimates"

	dateFormat = "2006-01-02"

	// Campus center, used when a nearby search gives no position.
	defaultLatitude  = 30.6138
	defaultLongitude = -96.3395
	defaultMinRadius = 1
	defaultMaxRadius = 20
)

// GetActiveRoutes returns the short names of routes running right now, such as "01" or "04".
func (c *Client) GetActiveRoutes(ctx context.Context) ([]string, error) {
	var out []string
	if err := c.postEmpty(ctx, pathActiveRoutes, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// NearbyRoutesQuery bounds a search for routes around a position.
// A zero value searches the default radii around campus center.
type NearbyRoutesQuery struct {
	FavoriteRoutes []string
	Latitude       float64
	Longitude      float64
	MinRadius      float64
	MaxRadius      float64
}

type nearbyRoutesPayload struct {
	Latitude        float64  `json:"latitude"`
	Longitude       float64  `json:"longitude"`
	MinRadius       float64  `json:"minRadius"`
	MaxRadius       float64  `json:"maxRadius"`
	FavouriteRoutes []string `json:"favouriteRoutes"`
}

func (q NearbyRoutesQuery) payload() nearbyRoutesPayload {
	p := nearbyRoutesPayload{
		Latitude:        q.Latitude,
		Longitude:       q.Longitude,
		MinRadius:       q.MinRadius,
		MaxRadius:       q.MaxRadius,
		FavouriteRoutes: q.FavoriteRoutes,
	}
	if p.Latitude == 0 && p.Longitude == 0 {
		p.Latitude, p.Longitude = defaultLatitude, defaultLongitude
	}
	if p.MinRadius == 0 {
		p.MinRadius = defaultMinRadius
	}
	if p.MaxRadius == 0 {
		p.MaxRadius = defaultMaxRadius
	}
	if p.FavouriteRoutes == nil {
		p.FavouriteRoutes = []string{}
	}
	return p
}

// GetNearbyRoutes returns routes with stops inside the searched radius.
func (c *Client) GetNearbyRoutes(ctx context.Context, query NearbyRoutesQuery) (*NearbyRoutesResponse, error) {
	var out NearbyRoutesResponse
	if err := c.postJSON(ctx, pathNearbyRoutes, query.payload(), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetNextStopTimes returns the upcoming stop times for each given route.
func (c *Client) GetNextStopTimes(ctx context.Context, routes []string) ([]TimetableRoute, error) {
	payload := struct {
		Routes []string `json:"routes"`
	}{Routes: routes}

	var out []TimetableRoute
	if err := c.postJSON(ctx, pathNextStopTimes, payload, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetStopSchedules returns the published timetable for a stop on a given day.
func (c *Client) GetStopSchedules(ctx context.Context, stopCode string, date time.Time) (*StopSchedulesResponse, error) {
	var out StopSchedulesResponse
	if err := c.postJSON(ctx, pathStopSchedules, stopDatePayload(stopCode, date), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetStopEstimates returns the realtime-adjusted timetable for a stop on a given day.
func (c *Client) GetStopEstimates(ctx context.Context, stopCode string, date time.Time) (*StopEstimatesResponse, error) {
	var out StopEstimatesResponse
	if err := c.postJSON(ctx, pathStopEstimates, stopDatePayload(stopCode, date), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func stopDatePayload(stopCode string, date time.Time) any {
	return struct {
		StopCode string `json:"stopCode"`
		Date     string `json:"date"`
	}{
		StopCode: stopCode,
		Date:     date.Format(dateFormat),
	}
}
