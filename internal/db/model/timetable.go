package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"github.com/MaroonRides/api/internal/db/sync"
)

type Timetable struct {
	bun.BaseModel `bun:"table:timetable"`
	sync.Syncable

	ID          uuid.UUID   `bun:"id,type:uuid,pk,default:uuidv7()"`
	StopID      uuid.UUID   `bun:"stopId,type:uuid,notnull,unique:timetable_slot_uq"`
	DirectionID uuid.UUID   `bun:"directionId,type:uuid,notnull,unique:timetable_slot_uq"`
	RouteID     uuid.UUID   `bun:"routeId,type:uuid,notnull"`
	ServiceDate time.Time   `bun:"serviceDate,type:date,notnull,unique:timetable_slot_uq"`
	Departures  []time.Time `bun:"departures,type:timestamptz[],array,notnull"`

	Stop      *Stop      `bun:"rel:belongs-to,join:stopId=id,on_delete:CASCADE"`
	Direction *Direction `bun:"rel:belongs-to,join:directionId=id,join:routeId=routeId,on_delete:CASCADE"`
}

type TimetableAudit struct {
	bun.BaseModel `bun:"table:timetable_audit"`
	sync.Tombstone

	TimetableID uuid.UUID `bun:"timetableId,type:uuid,notnull"`
}
