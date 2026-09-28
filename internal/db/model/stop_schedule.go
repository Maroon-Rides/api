package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"github.com/MaroonRides/api/internal/db/sync"
)

// StopSchedule is one published departure from a stop. Upstream only serves
// timetables per stop, with nothing linking a departure to the trip it is part of.
type StopSchedule struct {
	bun.BaseModel `bun:"table:stop_schedule"`
	sync.Syncable

	ID          uuid.UUID `bun:"id,type:uuid,pk,default:uuidv7()"`
	StopID      uuid.UUID `bun:"stopId,type:uuid,notnull,unique:stop_schedule_slot_uq"`
	DirectionID uuid.UUID `bun:"directionId,type:uuid,notnull,unique:stop_schedule_slot_uq"`
	ScheduledAt time.Time `bun:"scheduledAt,notnull,unique:stop_schedule_slot_uq"`
	ServiceDate time.Time `bun:"serviceDate,type:date,notnull"`

	Stop      *Stop      `bun:"rel:belongs-to,join:stopId=id,on_delete:CASCADE"`
	Direction *Direction `bun:"rel:belongs-to,join:directionId=id,on_delete:CASCADE"`
}

type StopScheduleAudit struct {
	bun.BaseModel `bun:"table:stop_schedule_audit"`
	sync.Tombstone

	StopScheduleID uuid.UUID `bun:"stopScheduleId,type:uuid,notnull"`
}
