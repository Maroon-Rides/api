package model

import (
	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"github.com/MaroonRides/api/internal/db/sync"
)

// Stop is where one direction stops. Upstream reuses a stop code across routes
// for stops with different names and locations, so a code alone is not a stop.
type Stop struct {
	bun.BaseModel `bun:"table:stop"`
	sync.Syncable

	ID          uuid.UUID `bun:"id,type:uuid,pk,default:uuidv7()"`
	DirectionID uuid.UUID `bun:"directionId,type:uuid,notnull,unique:stop_direction_source_uq"`
	SourceID    string    `bun:"sourceId,notnull,unique:stop_direction_source_uq" sync:"nosync"`

	Name        string   `bun:"name,notnull"`
	Lat         float64  `bun:"lat,notnull"`
	Lon         float64  `bun:"lon,notnull"`
	Amenities   []string `bun:"amenities,notnull"`
	Sequence    int      `bun:"sequence,notnull"`
	IsTimepoint bool     `bun:"isTimepoint,notnull,default:false"`

	Direction *Direction `bun:"rel:belongs-to,join:directionId=id,on_delete:CASCADE"`
}

type StopAudit struct {
	bun.BaseModel `bun:"table:stop_audit"`
	sync.Tombstone

	StopID uuid.UUID `bun:"stopId,type:uuid,notnull"`
}
