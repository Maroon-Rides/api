package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"github.com/MaroonRides/api/internal/db/sync"
)

type Alert struct {
	bun.BaseModel `bun:"table:alert"`
	sync.Syncable

	ID       uuid.UUID `bun:"id,type:uuid,pk,default:uuidv7()"`
	SourceID string    `bun:"sourceId,notnull,unique" sync:"nosync"`

	Title       string `bun:"title,notnull"`
	Description string `bun:"description,notnull"`

	TimeRangeText  string `bun:"timeRangeText,notnull"`
	DailyStartTime string `bun:"dailyStartTime,notnull"`
	DailyEndTime   string `bun:"dailyEndTime,notnull"`

	StartsAt time.Time  `bun:"startsAt,notnull"`
	EndsAt   *time.Time `bun:"endsAt,nullzero"`
}

type AlertAudit struct {
	bun.BaseModel `bun:"table:alert_audit"`
	sync.Tombstone

	AlertID uuid.UUID `bun:"alertId,type:uuid,notnull"`
}
