package model

import (
	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"github.com/MaroonRides/api/internal/db/sync"
)

type DirectionStop struct {
	bun.BaseModel `bun:"table:direction_stop"`
	sync.Syncable

	ID          uuid.UUID `bun:"id,type:uuid,pk,default:uuidv7()"`
	DirectionID uuid.UUID `bun:"directionId,type:uuid,notnull,unique:direction_stop_uq"`
	StopID      uuid.UUID `bun:"stopId,type:uuid,notnull,unique:direction_stop_uq"`

	Sequence    int  `bun:"sequence,notnull"`
	IsTimepoint bool `bun:"isTimepoint,notnull,default:false"`

	Direction *Direction `bun:"rel:belongs-to,join:directionId=id,on_delete:CASCADE"`
	Stop      *Stop      `bun:"rel:belongs-to,join:stopId=id,on_delete:CASCADE"`
}

type DirectionStopAudit struct {
	bun.BaseModel `bun:"table:direction_stop_audit"`
	sync.Tombstone

	DirectionStopID uuid.UUID `bun:"directionStopId,type:uuid,notnull"`
}
