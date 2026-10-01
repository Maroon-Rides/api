package busapi

import (
	"context"
	"net/url"
	"strconv"
	"strings"

	"github.com/samber/lo"
)

const (
	pathBaseData        = "/RouteMap/GetBaseData"
	pathPatternPaths    = "/RouteMap/GetPatternPaths"
	pathVehicles        = "/RouteMap/GetVehicles"
	pathNextDepartTimes = "/RouteMap/GetNextDepartTimes"
)

// GetBaseData returns every route and service interruption the map draws.
func (c *Client) GetBaseData(ctx context.Context) (*BaseDataResponse, error) {
	var out BaseDataResponse
	if err := c.postEmpty(ctx, pathBaseData, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetPatternPaths returns the drawn path and active vehicles for each given pattern.
func (c *Client) GetPatternPaths(ctx context.Context, patternIDs []string) ([]PatternPathsResponse, error) {
	var out []PatternPathsResponse
	if err := c.postForm(ctx, pathPatternPaths, encodeRouteKeys(patternIDs), &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetVehicles returns the active vehicles on the given patterns.
func (c *Client) GetVehicles(ctx context.Context, patternIDs []string) ([]VehicleResponse, error) {
	var out []VehicleResponse
	if err := c.postForm(ctx, pathVehicles, encodeRouteKeys(patternIDs), &out); err != nil {
		return nil, err
	}
	return out, nil
}

type RouteDirectionPair struct {
	RouteKey     string
	DirectionKey string
}

// GetNextDepartureTimes returns upcoming departures from a stop for each direction of a route.
func (c *Client) GetNextDepartureTimes(ctx context.Context, directions []RouteDirectionPair, stopCode string) (*NextDepartureTimesResponse, error) {
	parts := lo.Map(directions, func(pair RouteDirectionPair, i int) string {
		index := url.QueryEscape("routeDirectionKeys[" + strconv.Itoa(i) + "]")
		return index + url.QueryEscape("[routeKey]") + "=" + url.QueryEscape(pair.RouteKey) +
			"&" + index + url.QueryEscape("[directionKey]") + "=" + url.QueryEscape(pair.DirectionKey) +
			"&stopCode=" + url.QueryEscape(stopCode)
	})

	var out NextDepartureTimesResponse
	if err := c.postForm(ctx, pathNextDepartTimes, strings.Join(parts, "&"), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func encodeRouteKeys(ids []string) string {
	key := url.QueryEscape("routeKeys[]")
	parts := lo.Map(ids, func(id string, _ int) string { return key + "=" + url.QueryEscape(id) })
	return strings.Join(parts, "&")
}
