package busapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const (
	pathFindBusStops = "/Home/FindBusStops"
	pathTripPlan     = "/TripPlanner/GetTripPlan"
)

// RouteOption selects which trip the planner optimizes for.
type RouteOption int

// FindBusStops returns the bus stops matching a search query.
func (c *Client) FindBusStops(ctx context.Context, query string) ([]FoundStop, error) {
	var out []FoundStop
	err := c.do(ctx, request{
		method: http.MethodGet,
		path:   pathFindBusStops,
		query:  url.Values{"searchTerm": {query}},
		accept: acceptAjax,
	}, &out)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// TripPlanQuery describes the trip to plan. At most one of ArriveBy and DepartAt is meaningful;
// leaving both unset plans from now.
type TripPlanQuery struct {
	Origin      Endpoint
	Destination Endpoint
	RouteOption RouteOption
	ArriveBy    *time.Time
	DepartAt    *time.Time
}

type tripPlanPayload struct {
	Origin            string `json:"origin"`
	OriginName1       string `json:"originName1"`
	OriginName2       string `json:"originName2"`
	OriginLatitude    any    `json:"originLatitude"`
	OriginLongitude   any    `json:"originLongitude"`
	OriginStopCode    any    `json:"originStopCode"`
	OriginPlaceID     any    `json:"originPlaceId"`
	OriginFavourited  bool   `json:"originFavourited"`
	OriginGeolocation bool   `json:"originGeolocation"`

	Destination            string `json:"destination"`
	DestinationName1       string `json:"destinationName1"`
	DestinationName2       string `json:"destinationName2"`
	DestinationLatitude    any    `json:"destinationLatitude"`
	DestinationLongitude   any    `json:"destinationLongitude"`
	DestinationStopCode    any    `json:"destinationStopCode"`
	DestinationPlaceID     any    `json:"destinationPlaceId"`
	DestinationFavourited  bool   `json:"destinationFavourited"`
	DestinationGeolocation bool   `json:"destinationGeolocation"`

	ArriveTime                 *string     `json:"arriveTime,omitempty"`
	DepartTime                 *string     `json:"departTime,omitempty"`
	RouteOption                RouteOption `json:"routeOption"`
	IsOriginStopCodeValid      bool        `json:"isOriginStopCodeValid"`
	IsDestinationStopCodeValid bool        `json:"isDestinationStopCodeValid"`
	Lang                       *string     `json:"lang"`
}

func (q TripPlanQuery) payload() tripPlanPayload {
	return tripPlanPayload{
		Origin:          q.Origin.Title,
		OriginName1:     q.Origin.Title,
		OriginName2:     q.Origin.Subtitle,
		OriginLatitude:  orEmptyString(q.Origin.Latitude),
		OriginLongitude: orEmptyString(q.Origin.Longitude),
		OriginStopCode:  orEmptyString(q.Origin.StopCode),
		OriginPlaceID:   orEmptyString(q.Origin.PlaceID),

		Destination:          q.Destination.Title,
		DestinationName1:     q.Destination.Title,
		DestinationName2:     q.Destination.Subtitle,
		DestinationLatitude:  orEmptyString(q.Destination.Latitude),
		DestinationLongitude: orEmptyString(q.Destination.Longitude),
		DestinationStopCode:  orEmptyString(q.Destination.StopCode),
		DestinationPlaceID:   orEmptyString(q.Destination.PlaceID),

		ArriveTime:                 unixSeconds(q.ArriveBy),
		DepartTime:                 unixSeconds(q.DepartAt),
		RouteOption:                q.RouteOption,
		IsOriginStopCodeValid:      true,
		IsDestinationStopCodeValid: true,
	}
}

// GetTripPlan returns the planner's transit options between two endpoints.
func (c *Client) GetTripPlan(ctx context.Context, query TripPlanQuery) (*TripPlan, error) {
	body, err := json.Marshal(query.payload())
	if err != nil {
		return nil, fmt.Errorf("aggiespirit: encode %s: %w", pathTripPlan, err)
	}

	var out TripPlan
	err = c.do(ctx, request{
		method:      http.MethodPost,
		path:        pathTripPlan,
		contentType: contentTypeJSON,
		accept:      acceptAjax,
		body:        body,
	}, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// orEmptyString mirrors the planner's `value ?? ""`: it sends an empty string
// where the caller left a field unset, never null.
func orEmptyString[T any](v *T) any {
	if v == nil {
		return ""
	}
	return *v
}

func unixSeconds(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := strconv.FormatInt(t.Unix(), 10)
	return &s
}
