package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"github.com/MaroonRides/api/internal/db/sync"
)

type Route struct {
	bun.BaseModel `bun:"table:route"`
	sync.Syncable

	ID       uuid.UUID `bun:"id,type:uuid,pk,default:uuidv7()"`
	SourceID string    `bun:"sourceId,notnull,unique" sync:"nosync"`

	ShortName     string     `bun:"shortName,notnull"`
	LongName      string     `bun:"longName,notnull"`
	LightColor    string     `bun:"lightColor,notnull"`
	DarkColor     string     `bun:"darkColor,notnull"`
	Active        bool       `bun:"active,notnull,default:true"`
	DeactivatedAt *time.Time `bun:"deactivatedAt" sync:"nosync"`
}

type RouteAudit struct {
	bun.BaseModel `bun:"table:route_audit"`
	sync.Tombstone

	RouteID uuid.UUID `bun:"routeId,type:uuid,notnull"`
}
