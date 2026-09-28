package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

type Vehicle struct {
	bun.BaseModel `bun:"table:vehicle"`

	ID       uuid.UUID `bun:"id,type:uuid,pk,default:uuidv7()"`
	SourceID string    `bun:"sourceId,notnull,unique"`

	RouteID     uuid.UUID `bun:"routeId,type:uuid,notnull"`
	DirectionID uuid.UUID `bun:"directionId,type:uuid,notnull"`

	Name    string  `bun:"name,notnull"`
	Lat     float64 `bun:"lat,notnull"`
	Lon     float64 `bun:"lon,notnull"`
	Heading float64 `bun:"heading,notnull"`
	Speed   float64 `bun:"speed,notnull"`

	Passengers int `bun:"passengers,notnull"`
	Capacity   int `bun:"capacity,notnull"`

	Amenities []string  `bun:"amenities,notnull"`
	SeenAt    time.Time `bun:"seenAt,notnull,default:clock_timestamp()"`

	Route     *Route     `bun:"rel:belongs-to,join:routeId=id,on_delete:CASCADE"`
	Direction *Direction `bun:"rel:belongs-to,join:directionId=id,on_delete:CASCADE"`
}
