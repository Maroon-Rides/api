package test

import (
	"context"
	"time"

	"github.com/google/uuid"
	. "github.com/onsi/gomega"
	"github.com/uptrace/bun"

	"github.com/MaroonRides/api/internal/db/model"
)

// Network is one route with every synced table below it filled in.
type Network struct {
	Route          model.Route
	Direction      model.Direction
	Stop           model.Stop
	DirectionStop  model.DirectionStop
	Alert          model.Alert
	AlertDirection model.AlertDirection
	Timetable      model.Timetable
}

// CreateNetwork fills every synced table. The name keeps unique columns apart across calls.
func CreateNetwork(bundb *bun.DB, name string) Network {
	n := Network{
		Route: model.Route{
			SourceID:   "route-" + name,
			ShortName:  name,
			LongName:   "Route " + name,
			LightColor: "#500000",
			DarkColor:  "#ffffff",
			Active:     true,
		},
		Stop: model.Stop{
			SourceID:  "stop-" + name,
			Name:      "Stop " + name,
			Lat:       30.6187,
			Lon:       -96.3365,
			Amenities: []string{"shelter"},
		},
		Alert: model.Alert{
			SourceID:       "alert-" + name,
			Title:          "Detour " + name,
			Description:    "Construction",
			TimeRangeText:  "All day",
			DailyStartTime: "00:00",
			DailyEndTime:   "23:59",
			StartsAt:       time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
		},
	}
	Insert(bundb, &n.Route)
	Insert(bundb, &n.Stop)
	Insert(bundb, &n.Alert)

	n.Direction = model.Direction{
		RouteID:     n.Route.ID,
		SourceID:    "direction-" + name,
		Destination: "Downtown",
		Sequence:    1,
		Path:        "_p~iF~ps|U_ulLnnqC",
	}
	Insert(bundb, &n.Direction)

	n.DirectionStop = model.DirectionStop{
		DirectionID: n.Direction.ID,
		StopID:      n.Stop.ID,
		Sequence:    1,
		IsTimepoint: true,
	}
	n.AlertDirection = model.AlertDirection{AlertID: n.Alert.ID, DirectionID: n.Direction.ID}
	n.Timetable = model.Timetable{
		StopID:      n.Stop.ID,
		DirectionID: n.Direction.ID,
		RouteID:     n.Route.ID,
		ServiceDate: time.Date(2026, 9, 27, 0, 0, 0, 0, time.UTC),
		Departures: []time.Time{
			time.Date(2026, 9, 27, 14, 30, 0, 0, time.UTC),
			time.Date(2026, 9, 27, 14, 45, 0, 0, time.UTC),
		},
	}
	Insert(bundb, &n.DirectionStop)
	Insert(bundb, &n.AlertDirection)
	Insert(bundb, &n.Timetable)

	return n
}

func Insert[M any](bundb *bun.DB, row *M) {
	_, err := bundb.NewInsert().Model(row).Returning("*").Exec(context.Background())
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
}

func Exec(bundb *bun.DB, query string, args ...any) {
	_, err := bundb.ExecContext(context.Background(), query, args...)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
}

// UUIDv7Ago mints a uuidv7 on the database clock as if it were minted `ago` in the past.
func UUIDv7Ago(bundb *bun.DB, ago time.Duration) uuid.UUID {
	var id uuid.UUID
	err := bundb.NewRaw("SELECT uuidv7(-make_interval(secs => ?))", ago.Seconds()).Scan(context.Background(), &id)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	return id
}
