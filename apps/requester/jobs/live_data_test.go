package jobs

import (
	"testing"
	"time"

	"github.com/MaroonRides/api/apps/requester/busapi"
)

func TestParseUpstreamTimeTreatsZonelessAsUTC(t *testing.T) {
	want := time.Date(2026, 9, 27, 19, 30, 0, 0, time.UTC)

	for _, value := range []string{"2026-09-27T19:30:00Z", "2026-09-27T19:30:00", "2026-09-27T14:30:00-05:00"} {
		got, err := parseUpstreamTime(value)
		if err != nil {
			t.Fatalf("parseUpstreamTime(%q): %v", value, err)
		}
		if !got.Equal(want) {
			t.Errorf("parseUpstreamTime(%q) = %v, want %v", value, got, want)
		}
	}
}

func TestDepartureRowRequiresScheduledTime(t *testing.T) {
	estimated := "2026-09-27T19:30:00Z"

	if _, err := departureRow(busapi.DepartureTime{EstimatedDepartTimeUtc: &estimated}); err == nil {
		t.Error("expected an error for a departure with no scheduled time")
	}
}

func TestDepartureRowLeavesEstimateEmptyWhenMissing(t *testing.T) {
	scheduled := "2026-09-27T19:30:00Z"

	departure, err := departureRow(busapi.DepartureTime{ScheduledDepartTimeUtc: &scheduled})
	if err != nil {
		t.Fatalf("departureRow: %v", err)
	}
	if departure.EstimatedAt != nil {
		t.Errorf("EstimatedAt = %v, want nil", departure.EstimatedAt)
	}
}
