package jobs

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/MaroonRides/api/apps/requester/busapi"
	"github.com/MaroonRides/api/internal/db/model"
)

func TestServiceDateUsesServiceTimeZone(t *testing.T) {
	location, err := NewServiceLocation()
	if err != nil {
		t.Fatal(err)
	}

	// 03:00 UTC is still the previous evening in College Station
	got := serviceDate(time.Date(2026, 9, 28, 3, 0, 0, 0, time.UTC), location)

	if got.Format(time.DateOnly) != "2026-09-27" {
		t.Errorf("serviceDate = %s, want 2026-09-27", got.Format(time.DateOnly))
	}
}

func testScheduleIndex() (scheduleDirectionIndex, model.Direction, model.Direction) {
	toMSC := model.Direction{ID: uuid.New(), SourceID: "to-msc"}
	circulator := model.Direction{ID: uuid.New(), SourceID: "circulator"}

	routes := []busapi.MapRoute{
		{ShortName: "03", DirectionList: []busapi.MapDirectionList{
			{Direction: busapi.MapDirection{Key: "to-msc", Name: "to MSC"}},
			{Direction: busapi.MapDirection{Key: "to-white-creek", Name: "to White Creek"}},
		}},
		{ShortName: "01", DirectionList: []busapi.MapDirectionList{
			{Direction: busapi.MapDirection{Key: "circulator", Name: "Campus Circulator"}},
		}},
	}

	index := newScheduleDirectionIndex(routes, map[string]model.Direction{
		toMSC.SourceID:      toMSC,
		circulator.SourceID: circulator,
	})

	return index, toMSC, circulator
}

func TestScheduleDirectionLookup(t *testing.T) {
	index, toMSC, circulator := testScheduleIndex()

	tests := []struct {
		name          string
		routeNumber   string
		directionName string
		want          model.Direction
		found         bool
	}{
		{"matches by name", "03", "to MSC", toMSC, true},
		{"matches single direction by name", "01", "Campus Circulator", circulator, true},
		{"unstored direction", "03", "to White Creek", model.Direction{}, false},
		{"unknown route", "99", "to MSC", model.Direction{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, found := index.lookup(tt.routeNumber, tt.directionName)
			if found != tt.found || got.ID != tt.want.ID {
				t.Errorf("lookup(%q, %q) = %s, %v; want %s, %v", tt.routeNumber, tt.directionName, got.ID, found, tt.want.ID, tt.found)
			}
		})
	}
}

func TestStopScheduleRowsSkipsUnknownDirections(t *testing.T) {
	index, toMSC, _ := testScheduleIndex()
	stop := model.Stop{ID: uuid.New(), SourceID: "0048"}
	date := time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)

	results := map[string]busapi.StopSchedulesResponse{
		"0048": {RouteStopSchedules: []busapi.RouteStopSchedule{
			{RouteNumber: "03", DirectionName: "to MSC", StopTimes: []busapi.StopTime{
				{ScheduledDepartTimeUtc: "2026-09-28T12:11:00Z"},
			}},
			{RouteNumber: "47", DirectionName: "Inbound", StopTimes: []busapi.StopTime{
				{ScheduledDepartTimeUtc: "2026-09-28T12:43:00Z"},
			}},
		}},
	}

	rows := stopScheduleRows(results, map[string]model.Stop{stop.SourceID: stop}, index, date)

	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(rows))
	}
	if rows[0].DirectionID != toMSC.ID || rows[0].StopID != stop.ID {
		t.Errorf("got %+v, want stop %s on direction %s", rows[0], stop.ID, toMSC.ID)
	}
}
