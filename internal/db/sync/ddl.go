package sync

import (
	"context"
	"fmt"
	"strings"

	"github.com/samber/lo"
	"github.com/uptrace/bun"
)

const reconcileLock = 0x53594e43

const bumpFunction = `
CREATE OR REPLACE FUNCTION sync_bump_update_id() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
  NEW."updatedAt" := clock_timestamp();
  NEW."updateId" := uuidv7();
  RETURN NEW;
END;
$$;`

const tombstoneTemplate = `
CREATE OR REPLACE FUNCTION %[1]s_sync_tombstone() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
  INSERT INTO "%[3]s" (%[4]s) SELECT %[2]s FROM deleted;
  RETURN NULL;
END;
$$;

DROP TRIGGER IF EXISTS %[1]s_sync_bump ON "%[1]s";
CREATE TRIGGER %[1]s_sync_bump
  BEFORE UPDATE ON "%[1]s"
  FOR EACH ROW WHEN (%[5]s)
  EXECUTE FUNCTION sync_bump_update_id();

DROP TRIGGER IF EXISTS %[1]s_sync_tombstone ON "%[1]s";
CREATE TRIGGER %[1]s_sync_tombstone
  AFTER DELETE ON "%[1]s"
  REFERENCING OLD TABLE AS deleted
  FOR EACH STATEMENT EXECUTE FUNCTION %[1]s_sync_tombstone();

DROP TRIGGER IF EXISTS %[1]s_sync_freeze_scope ON "%[1]s";`

const freezeScopeTemplate = `
CREATE OR REPLACE FUNCTION %[1]s_sync_freeze_scope() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
  RAISE EXCEPTION '%[1]s.%[2]s cannot change once written';
END;
$$;

CREATE TRIGGER %[1]s_sync_freeze_scope
  BEFORE UPDATE OF "%[2]s" ON "%[1]s"
  FOR EACH ROW WHEN (OLD."%[2]s" IS DISTINCT FROM NEW."%[2]s")
  EXECUTE FUNCTION %[1]s_sync_freeze_scope();`

func TriggerDDL(t Table) string {
	sourceColumns, auditColumns := quoted(t.Key), quoted(t.AuditKey)
	if t.Scope != "" {
		sourceColumns, auditColumns = quoted(t.Key, t.Scope), quoted(t.AuditKey, t.Scope)
	}

	ddl := fmt.Sprintf(tombstoneTemplate, t.Name, sourceColumns, t.Audit, auditColumns, changedRow(t))
	if t.Scope != "" {
		ddl += fmt.Sprintf(freezeScopeTemplate, t.Name, t.Scope)
	}
	return ddl
}

func quoted(columns ...string) string {
	return strings.Join(lo.Map(columns, func(c string, _ int) string { return fmt.Sprintf("%q", c) }), ", ")
}

// changedRow compares every column the model did not tag nosync, so rewriting a
// server-only column does not mint an updateId and push an identical row to
// every client.
func changedRow(t Table) string {
	columns := func(prefix string) string {
		return strings.Join(lo.Map(t.Synced, func(c string, _ int) string {
			return fmt.Sprintf("%s.%q", prefix, c)
		}), ", ")
	}
	return fmt.Sprintf("(%s) IS DISTINCT FROM (%s)", columns("OLD"), columns("NEW"))
}

func IndexDDL(ts []Table) string {
	var b strings.Builder
	for _, t := range ts {
		// Kept on scoped tables too, since an older unscoped version of a stream may still read the whole table.
		fmt.Fprintf(&b, "CREATE INDEX %[1]q ON %[2]q (%[3]q);\n", t.Name+"_updateId_idx", t.Name, UpsertColumn)
		if t.Scope != "" {
			fmt.Fprintf(&b, "CREATE INDEX %[1]q ON %[2]q (%[3]q, %[4]q);\n", t.Name+"_"+t.Scope+"_updateId_idx", t.Name, t.Scope, UpsertColumn)
			fmt.Fprintf(&b, "CREATE INDEX %[1]q ON %[2]q (%[3]q, %[4]q);\n", t.Audit+"_"+t.Scope+"_id_idx", t.Audit, t.Scope, DeleteColumn)
		}
		fmt.Fprintf(&b, "CREATE INDEX %[1]q ON %[2]q (\"deletedAt\");\n", t.Audit+"_deletedAt_idx", t.Audit)
	}
	return b.String()
}

func Reconcile(ctx context.Context, db *bun.DB, ts []Table) error {
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock(?)", reconcileLock); err != nil {
			return fmt.Errorf("take reconcile lock: %w", err)
		}
		if _, err := tx.ExecContext(ctx, bumpFunction); err != nil {
			return fmt.Errorf("create sync_bump_update_id: %w", err)
		}
		for _, t := range ts {
			if _, err := tx.ExecContext(ctx, TriggerDDL(t)); err != nil {
				return fmt.Errorf("reconcile triggers on %s: %w", t.Name, err)
			}
		}
		return nil
	})
}
