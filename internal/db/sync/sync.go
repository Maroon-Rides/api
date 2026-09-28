package sync

import (
	"fmt"
	"reflect"
	"regexp"
	"time"

	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/schema"
)

// A column tagged sync:"nosync" stays off the stream. Every other column on a
// Syncable model rides it, so a missing tag publishes a column rather than
// hiding one.
const (
	Tag    = "sync"
	NoSync = "nosync"
)

type Syncable struct {
	UpdatedAt time.Time `bun:"updatedAt,notnull,default:clock_timestamp()" sync:"nosync"`
	UpdateID  uuid.UUID `bun:"updateId,type:uuid,notnull,default:uuidv7()" sync:"nosync"`
}

type Tombstone struct {
	ID        uuid.UUID `bun:"id,type:uuid,pk,default:uuidv7()"`
	DeletedAt time.Time `bun:"deletedAt,notnull,default:clock_timestamp()"`
}

// Row is a synced row or tombstone, positioned in its stream by SyncID.
type Row interface {
	SyncID() uuid.UUID
}

func (s Syncable) SyncID() uuid.UUID  { return s.UpdateID }
func (t Tombstone) SyncID() uuid.UUID { return t.ID }

const (
	UpsertColumn = "updateId"
	DeleteColumn = "id"
)

type Table struct {
	Name       string
	Key        string
	Audit      string
	AuditKey   string
	Synced     []string
	Model      any
	AuditModel any
}

var (
	tables = schema.NewTables(pgdialect.New())
	ident  = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)
)

func For[M, A any](auditKey string) Table {
	model, audit := new(M), new(A)
	mt, at := describe(model), describe(audit)

	requireEmbed[M](reflect.TypeFor[Syncable]())
	requireEmbed[A](reflect.TypeFor[Tombstone]())

	if len(mt.PKs) != 1 {
		panic(fmt.Sprintf("sync: %s needs exactly one primary key column, has %d", mt.Name, len(mt.PKs)))
	}
	if _, ok := at.FieldMap[auditKey]; !ok {
		panic(fmt.Sprintf("sync: %s has no column %q", at.Name, auditKey))
	}

	t := Table{
		Name:       mt.Name,
		Key:        mt.PKs[0].Name,
		Audit:      at.Name,
		AuditKey:   auditKey,
		Synced:     syncedColumns(mt),
		Model:      model,
		AuditModel: audit,
	}
	if len(t.Synced) == 0 {
		panic(fmt.Sprintf("sync: %s tags every column %q, leaving nothing to sync", mt.Name, NoSync))
	}
	for _, name := range append([]string{t.Name, t.Key, t.Audit, t.AuditKey}, t.Synced...) {
		if !ident.MatchString(name) {
			panic(fmt.Sprintf("sync: %q is not a plain identifier", name))
		}
	}
	return t
}

func Models(ts []Table) []any {
	return lo.FlatMap(ts, func(t Table, _ int) []any { return []any{t.Model, t.AuditModel} })
}

func syncedColumns(t *schema.Table) []string {
	return lo.FilterMap(t.Fields, func(f *schema.Field, _ int) (string, bool) {
		return f.Name, !Ignored(f.StructField)
	})
}

func Ignored(f reflect.StructField) bool {
	switch tag := f.Tag.Get(Tag); tag {
	case "":
		return false
	case NoSync:
		return true
	default:
		panic(fmt.Sprintf("sync: %s has tag %s:%q, want %q", f.Name, Tag, tag, NoSync))
	}
}

func describe(model any) *schema.Table {
	return tables.Get(reflect.TypeOf(model).Elem())
}

func requireEmbed[T any](want reflect.Type) {
	typ := reflect.TypeFor[T]()
	for f := range typ.Fields() {
		if f.Anonymous && f.Type == want {
			return
		}
	}
	panic(fmt.Sprintf("sync: %s does not embed sync.%s", typ.Name(), want.Name()))
}
