package model

import (
	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

type SyncMetadataKey string

var SyncMetadataKeys = struct {
	// ResetBefore makes every client whose last complete sync is older start over. A migration raises it:
	// INSERT INTO "sync_metadata" ("key", "value") VALUES ('reset_before', uuidv7(INTERVAL '1 minute'))
	// ON CONFLICT ("key") DO UPDATE SET "value" = EXCLUDED."value";
	ResetBefore SyncMetadataKey
}{
	ResetBefore: "reset_before",
}

type SyncMetadata struct {
	bun.BaseModel `bun:"table:sync_metadata"`

	Key   SyncMetadataKey `bun:"key,pk"`
	Value uuid.UUID       `bun:"value,type:uuid,notnull"`
}
