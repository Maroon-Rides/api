package model

import (
	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"github.com/MaroonRides/api/internal/db/sync"
)

type Stop struct {
	bun.BaseModel `bun:"table:stop"`
	sync.Syncable

	ID       uuid.UUID `bun:"id,type:uuid,pk,default:uuidv7()"`
	SourceID string    `bun:"sourceId,notnull,unique" sync:"nosync"`

	Name      string   `bun:"name,notnull"`
	Lat       float64  `bun:"lat,notnull"`
	Lon       float64  `bun:"lon,notnull"`
	Amenities []string `bun:"amenities,notnull"`
}

type StopAudit struct {
	bun.BaseModel `bun:"table:stop_audit"`
	sync.Tombstone

	StopID uuid.UUID `bun:"stopId,type:uuid,notnull"`
}
