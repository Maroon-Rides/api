package model

import (
	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"github.com/MaroonRides/api/internal/db/sync"
)

type Direction struct {
	bun.BaseModel `bun:"table:direction"`
	sync.Syncable

	ID       uuid.UUID `bun:"id,type:uuid,pk,default:uuidv7()"`
	RouteID  uuid.UUID `bun:"routeId,type:uuid,notnull,unique:direction_route_destination_uq"`
	SourceID string    `bun:"sourceId,notnull,unique" sync:"nosync"`

	Destination string `bun:"destination,notnull,unique:direction_route_destination_uq"`
	Sequence    int    `bun:"sequence,notnull"`
	Path        string `bun:"path,notnull"` // Google Polyline encoded

	Route *Route `bun:"rel:belongs-to,join:routeId=id,on_delete:CASCADE"`
}

type DirectionAudit struct {
	bun.BaseModel `bun:"table:direction_audit"`
	sync.Tombstone

	DirectionID uuid.UUID `bun:"directionId,type:uuid,notnull"`
}
