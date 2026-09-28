package sync

import (
	"time"

	"github.com/google/uuid"
)

// A client that last finished a sync before ResetAfter may have missed tombstones
// that were already pruned, so it has to start over.
const (
	ResetAfter         = 30 * 24 * time.Hour
	TombstoneRetention = ResetAfter + 24*time.Hour
)

func Expired(id uuid.UUID, now time.Time) bool {
	minted, ok := MintedAt(id)
	return !ok || minted.Before(now.Add(-ResetAfter))
}

func MintedAt(u uuid.UUID) (time.Time, bool) {
	if u.Version() != 7 {
		return time.Time{}, false
	}
	var ms int64
	for _, b := range u[:6] {
		ms = ms<<8 | int64(b)
	}
	return time.UnixMilli(ms), true
}
