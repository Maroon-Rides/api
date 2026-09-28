package jobs

import (
	"testing"

	"github.com/google/uuid"

	"github.com/MaroonRides/api/apps/requester/busapi"
	"github.com/MaroonRides/api/internal/db/model"
)

func TestAlertRowsLeavesEndEmptyUntilFurtherNotice(t *testing.T) {
	rows := alertRows([]busapi.MapServiceInterruption{
		{Key: "3579", StartDateUtc: "2026-09-03T17:00:00+00:00", EndDateUtc: ""},
	})

	if len(rows) != 1 {
		t.Fatalf("got %d alerts, want 1", len(rows))
	}
	if rows[0].EndsAt != nil {
		t.Errorf("EndsAt = %v, want nil", rows[0].EndsAt)
	}
}

func TestAlertRowsSkipsUnparseableStart(t *testing.T) {
	rows := alertRows([]busapi.MapServiceInterruption{{Key: "1", StartDateUtc: "soon"}})

	if len(rows) != 0 {
		t.Errorf("got %d alerts, want 0", len(rows))
	}
}

func TestAlertDirectionRowsMatchesNumericKeys(t *testing.T) {
	alert := model.Alert{ID: uuid.New(), SourceID: "3582"}
	direction := model.Direction{ID: uuid.New(), SourceID: "inbound"}

	routes := []busapi.MapRoute{{
		DirectionList: []busapi.MapDirectionList{
			{Direction: busapi.MapDirection{Key: "inbound"}, ServiceInterruptionKeys: []int{3582, 9999}},
			{Direction: busapi.MapDirection{Key: "outbound"}, ServiceInterruptionKeys: []int{3582}},
		},
	}}

	rows := alertDirectionRows(routes,
		map[string]model.Alert{alert.SourceID: alert},
		map[string]model.Direction{direction.SourceID: direction},
	)

	if len(rows) != 1 {
		t.Fatalf("got %d alert directions, want 1", len(rows))
	}
	if rows[0].AlertID != alert.ID || rows[0].DirectionID != direction.ID {
		t.Errorf("got %+v, want alert %s on direction %s", rows[0], alert.ID, direction.ID)
	}
}
