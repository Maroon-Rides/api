package dtos

import (
	"encoding/json"

	"github.com/MaroonRides/api/internal/enum"
)

type ReportEventType string

var ReportEventTypes = struct {
	// The sync stream answered with an error status.
	SyncFailedHTTPStatusV1 ReportEventType
	// A line in the sync stream could not be read.
	SyncFailedInvalidLineV1 ReportEventType
	// The client could not write the lines it received to its database.
	SyncFailedApplyV1 ReportEventType
}{
	SyncFailedHTTPStatusV1:  "SyncFailedHTTPStatusV1",
	SyncFailedInvalidLineV1: "SyncFailedInvalidLineV1",
	SyncFailedApplyV1:       "SyncFailedApplyV1",
}

func (ReportEventType) EnumValues() []any { return enum.Values(ReportEventTypes) }

// A payload never changes once shipped. A new field means a new version of the event.
var ReportPayloads = map[ReportEventType]any{
	ReportEventTypes.SyncFailedHTTPStatusV1:  SyncFailedHTTPStatusV1{},
	ReportEventTypes.SyncFailedInvalidLineV1: SyncFailedInvalidLineV1{},
	ReportEventTypes.SyncFailedApplyV1:       SyncFailedApplyV1{},
}

type ReportRequest struct {
	Events []ReportEvent `json:"events"`
}

// Payload is decoded by the gateway once it knows the type.
type ReportEvent struct {
	Type    ReportEventType `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// Acks in every sync failure are the checkpoints the failed sync started from. None means the client's database was empty.

type SyncFailedHTTPStatusV1 struct {
	Protocol SyncProtocolVersion `json:"protocol"`
	Acks     []string            `json:"acks"`
	Status   int                 `json:"status"`
	Message  string              `json:"message,omitempty"`
}

type SyncFailedInvalidLineV1 struct {
	Protocol SyncProtocolVersion `json:"protocol"`
	Acks     []string            `json:"acks"`
	Message  string              `json:"message,omitempty"`
}

type SyncFailedApplyV1 struct {
	Protocol SyncProtocolVersion `json:"protocol"`
	Acks     []string            `json:"acks"`
	Message  string              `json:"message,omitempty"`
}
