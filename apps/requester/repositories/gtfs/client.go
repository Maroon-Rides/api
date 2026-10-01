package gtfs

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	defaultFeedURL = "https://transport.tamu.edu/webFS/gtfs/google_transit.zip"
	defaultTimeout = 60 * time.Second

	feedSizeLimit = 64 << 20

	fileRoutes    = "routes.txt"
	fileStops     = "stops.txt"
	fileTrips     = "trips.txt"
	fileStopTimes = "stop_times.txt"

	timepointExact       = "1"
	timepointUnspecified = ""

	// Some publishers start each file with a UTF-8 byte order mark, which csv keeps on the first header.
	byteOrderMark = "\xEF\xBB\xBF"
)

// Client downloads the Texas A&M static GTFS feed.
type Client struct {
	feedURL string
	http    *http.Client
}

type ClientConfig struct {
	// defaults to the Texas A&M Transportation Services feed
	FeedURL string

	HTTPClient *http.Client
}

func NewClient(cfg ClientConfig) *Client {
	c := &Client{feedURL: defaultFeedURL, http: cfg.HTTPClient}
	if cfg.FeedURL != "" {
		c.feedURL = cfg.FeedURL
	}
	if c.http == nil {
		c.http = &http.Client{Timeout: defaultTimeout}
	}
	return c
}

type Route struct {
	RouteID   string
	ShortName string
}

type Stop struct {
	StopID   string
	StopCode string
}

type Trip struct {
	TripID  string
	RouteID string
}

type StopTime struct {
	TripID    string
	StopID    string
	Timepoint bool
}

type Feed struct {
	Routes    []Route
	Stops     []Stop
	Trips     []Trip
	StopTimes []StopTime
}

func (c *Client) GetFeed(ctx context.Context) (*Feed, error) {
	archive, err := c.download(ctx)
	if err != nil {
		return nil, err
	}

	var feed Feed

	feed.Routes, err = readTable(archive, fileRoutes, func(row csvRow) Route {
		return Route{RouteID: row.get("route_id"), ShortName: row.get("route_short_name")}
	})
	if err != nil {
		return nil, err
	}

	feed.Stops, err = readTable(archive, fileStops, func(row csvRow) Stop {
		return Stop{StopID: row.get("stop_id"), StopCode: row.get("stop_code")}
	})
	if err != nil {
		return nil, err
	}

	feed.Trips, err = readTable(archive, fileTrips, func(row csvRow) Trip {
		return Trip{TripID: row.get("trip_id"), RouteID: row.get("route_id")}
	})
	if err != nil {
		return nil, err
	}

	feed.StopTimes, err = readTable(archive, fileStopTimes, func(row csvRow) StopTime {
		return StopTime{
			TripID:    row.get("trip_id"),
			StopID:    row.get("stop_id"),
			Timepoint: isTimepoint(row.get("timepoint"), row.get("arrival_time")),
		}
	})
	if err != nil {
		return nil, err
	}

	return &feed, nil
}

// isTimepoint follows the GTFS rule that a blank timepoint means exact when the stop has a time.
func isTimepoint(timepoint, arrivalTime string) bool {
	return timepoint == timepointExact || (timepoint == timepointUnspecified && arrivalTime != "")
}

func (c *Client) download(ctx context.Context) (*zip.Reader, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.feedURL, nil)
	if err != nil {
		return nil, fmt.Errorf("gtfs: build request: %w", err)
	}

	res, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gtfs: download %s: %w", c.feedURL, err)
	}
	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("gtfs: download %s: %s", c.feedURL, res.Status)
	}

	body, err := io.ReadAll(io.LimitReader(res.Body, feedSizeLimit))
	if err != nil {
		return nil, fmt.Errorf("gtfs: read %s: %w", c.feedURL, err)
	}

	archive, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		return nil, fmt.Errorf("gtfs: open archive: %w", err)
	}
	return archive, nil
}

type csvRow struct {
	columns map[string]int
	record  []string
}

func (r csvRow) get(column string) string {
	i, ok := r.columns[column]
	if !ok || i >= len(r.record) {
		return ""
	}
	return r.record[i]
}

func readTable[T any](archive *zip.Reader, name string, decode func(csvRow) T) ([]T, error) {
	file, err := archive.Open(name)
	if err != nil {
		return nil, fmt.Errorf("gtfs: open %s: %w", name, err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1

	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("gtfs: read %s header: %w", name, err)
	}

	columns := make(map[string]int, len(header))
	for i, column := range header {
		columns[strings.TrimPrefix(column, byteOrderMark)] = i
	}

	var out []T
	for {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			return out, nil
		}
		if err != nil {
			return nil, fmt.Errorf("gtfs: read %s: %w", name, err)
		}
		out = append(out, decode(csvRow{columns: columns, record: record}))
	}
}
