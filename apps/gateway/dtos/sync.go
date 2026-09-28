package dtos

import "github.com/MaroonRides/api/internal/enum"

type SyncRequestType string

var SyncRequestTypes = struct {
	RoutesV1          SyncRequestType
	DirectionsV1      SyncRequestType
	StopsV1           SyncRequestType
	DirectionStopsV1  SyncRequestType
	AlertsV1          SyncRequestType
	AlertDirectionsV1 SyncRequestType
	StopSchedulesV1   SyncRequestType
}{
	RoutesV1:          "RoutesV1",
	DirectionsV1:      "DirectionsV1",
	StopsV1:           "StopsV1",
	DirectionStopsV1:  "DirectionStopsV1",
	AlertsV1:          "AlertsV1",
	AlertDirectionsV1: "AlertDirectionsV1",
	StopSchedulesV1:   "StopSchedulesV1",
}

func (SyncRequestType) EnumValues() []any { return enum.Values(SyncRequestTypes) }

type SyncEntityType string

var SyncEntityTypes = struct {
	RouteV1                SyncEntityType
	RouteDeleteV1          SyncEntityType
	DirectionV1            SyncEntityType
	DirectionDeleteV1      SyncEntityType
	StopV1                 SyncEntityType
	StopDeleteV1           SyncEntityType
	DirectionStopV1        SyncEntityType
	DirectionStopDeleteV1  SyncEntityType
	AlertV1                SyncEntityType
	AlertDeleteV1          SyncEntityType
	AlertDirectionV1       SyncEntityType
	AlertDirectionDeleteV1 SyncEntityType
	StopScheduleV1         SyncEntityType
	StopScheduleDeleteV1   SyncEntityType

	SyncResetV1    SyncEntityType // wipe every synced table and ack, then sync again
	SyncCompleteV1 SyncEntityType
}{
	RouteV1:                "RouteV1",
	RouteDeleteV1:          "RouteDeleteV1",
	DirectionV1:            "DirectionV1",
	DirectionDeleteV1:      "DirectionDeleteV1",
	StopV1:                 "StopV1",
	StopDeleteV1:           "StopDeleteV1",
	DirectionStopV1:        "DirectionStopV1",
	DirectionStopDeleteV1:  "DirectionStopDeleteV1",
	AlertV1:                "AlertV1",
	AlertDeleteV1:          "AlertDeleteV1",
	AlertDirectionV1:       "AlertDirectionV1",
	AlertDirectionDeleteV1: "AlertDirectionDeleteV1",
	StopScheduleV1:         "StopScheduleV1",
	StopScheduleDeleteV1:   "StopScheduleDeleteV1",

	SyncResetV1:    "SyncResetV1",
	SyncCompleteV1: "SyncCompleteV1",
}

func (SyncEntityType) EnumValues() []any { return enum.Values(SyncEntityTypes) }

type SyncRequest struct {
	Types []SyncRequestType `json:"types"`
	Acks  []string          `json:"acks"`
}

type SyncStreamLine struct {
	Type SyncEntityType `json:"type"`
	Ack  string         `json:"ack"`
	Data any            `json:"data"`
}
