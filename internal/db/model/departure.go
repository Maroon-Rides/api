package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

type Departure struct {
	bun.BaseModel `bun:"table:departure"`

	ID uuid.UUID `bun:"id,type:uuid,pk,default:uuidv7()"`

	RouteID     uuid.UUID `bun:"routeId,type:uuid,notnull,unique:departure_slot_uq"`
	StopID      uuid.UUID `bun:"stopId,type:uuid,notnull,unique:departure_slot_uq"`
	DirectionID uuid.UUID `bun:"directionId,type:uuid,notnull,unique:departure_slot_uq"`
	ScheduledAt time.Time `bun:"scheduledAt,notnull,unique:departure_slot_uq"`

	EstimatedAt *time.Time `bun:"estimatedAt,nullzero"`
	IsCancelled bool       `bun:"isCancelled,notnull"`

	Route     *Route     `bun:"rel:belongs-to,join:routeId=id,on_delete:CASCADE"`
	Stop      *Stop      `bun:"rel:belongs-to,join:stopId=id,on_delete:CASCADE"`
	Direction *Direction `bun:"rel:belongs-to,join:directionId=id,on_delete:CASCADE"`
}
