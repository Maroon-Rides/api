package model

import (
	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"github.com/MaroonRides/api/internal/db/sync"
)

// AlertDirection attaches an alert to a direction, which is the level upstream
// reports it at. The app shows alerts per route and rolls these up itself.
type AlertDirection struct {
	bun.BaseModel `bun:"table:alert_direction"`
	sync.Syncable

	ID          uuid.UUID `bun:"id,type:uuid,pk,default:uuidv7()"`
	AlertID     uuid.UUID `bun:"alertId,type:uuid,notnull,unique:alert_direction_uq"`
	DirectionID uuid.UUID `bun:"directionId,type:uuid,notnull,unique:alert_direction_uq"`

	Alert     *Alert     `bun:"rel:belongs-to,join:alertId=id,on_delete:CASCADE"`
	Direction *Direction `bun:"rel:belongs-to,join:directionId=id,on_delete:CASCADE"`
}

type AlertDirectionAudit struct {
	bun.BaseModel `bun:"table:alert_direction_audit"`
	sync.Tombstone

	AlertDirectionID uuid.UUID `bun:"alertDirectionId,type:uuid,notnull"`
}
