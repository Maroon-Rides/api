// Package notify publishes row changes over postgres LISTEN/NOTIFY.
package notify

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const reconcileLock = 0x4e4f5449

// LiveDataSubscriptionsChannel carries the routeId of a route's first subscription.
const LiveDataSubscriptionsChannel = "live_data_subscriptions"

// LiveDataAvailableChannel carries the routeId of a route whose first fetch finished.
const LiveDataAvailableChannel = "live_data_available"

// VehiclesChannel and DeparturesChannel carry the routeId of a route whose rows changed.
const (
	VehiclesChannel   = "vehicles_changed"
	DeparturesChannel = "departures_changed"
)

// Upserts that hit the conflict path are UPDATEs, so a refreshed subscription stays quiet.
// Two clients subscribing to the same route at once can't see each other's uncommitted
// rows, so both may notify: a duplicate is possible, a missed route is not.
var liveDataSubscriptionsDDL = fmt.Sprintf(`
CREATE OR REPLACE FUNCTION live_data_subscriptions_notify() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM "live_data_subscriptions"
    WHERE "routeId" = NEW."routeId" AND "id" <> NEW."id"
  ) THEN
    PERFORM pg_notify('%[1]s', NEW."routeId"::text);
  END IF;
  RETURN NULL;
END;
$$;

DROP TRIGGER IF EXISTS live_data_subscriptions_notify ON "live_data_subscriptions";
CREATE TRIGGER live_data_subscriptions_notify
  AFTER INSERT ON "live_data_subscriptions"
  FOR EACH ROW EXECUTE FUNCTION live_data_subscriptions_notify();`, LiveDataSubscriptionsChannel)

// A route's live data stops refreshing once its last subscriber leaves.
const liveDataUnavailableDDL = `
CREATE OR REPLACE FUNCTION live_data_subscriptions_unavailable() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
  UPDATE "route" SET "liveDataAvailable" = FALSE
  WHERE "liveDataAvailable"
    AND "id" IN (SELECT "routeId" FROM deleted)
    AND NOT EXISTS (
      SELECT 1 FROM "live_data_subscriptions" s WHERE s."routeId" = "route"."id"
    );
  RETURN NULL;
END;
$$;

DROP TRIGGER IF EXISTS live_data_subscriptions_unavailable ON "live_data_subscriptions";
CREATE TRIGGER live_data_subscriptions_unavailable
  AFTER DELETE ON "live_data_subscriptions"
  REFERENCING OLD TABLE AS deleted
  FOR EACH STATEMENT EXECUTE FUNCTION live_data_subscriptions_unavailable();`

var liveDataAvailableDDL = fmt.Sprintf(`
CREATE OR REPLACE FUNCTION live_data_available_notify() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
  PERFORM pg_notify('%[1]s', NEW."id"::text);
  RETURN NULL;
END;
$$;

DROP TRIGGER IF EXISTS live_data_available_notify ON "route";
CREATE TRIGGER live_data_available_notify
  AFTER UPDATE OF "liveDataAvailable" ON "route"
  FOR EACH ROW WHEN (NEW."liveDataAvailable" AND NOT OLD."liveDataAvailable")
  EXECUTE FUNCTION live_data_available_notify();`, LiveDataAvailableChannel)

// Postgres folds identical notifications within a transaction, so a sync that
// touches every row of a route still notifies that route once.
const routeRowsChangedFunctionDDL = `
CREATE OR REPLACE FUNCTION route_rows_changed_notify() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
  IF TG_OP IN ('UPDATE', 'DELETE') THEN
    PERFORM pg_notify(TG_ARGV[0], OLD."routeId"::text);
  END IF;
  IF TG_OP IN ('INSERT', 'UPDATE') THEN
    PERFORM pg_notify(TG_ARGV[0], NEW."routeId"::text);
  END IF;
  RETURN NULL;
END;
$$;`

func routeRowsChangedTriggerDDL(table, channel string) string {
	return fmt.Sprintf(`
DROP TRIGGER IF EXISTS route_rows_changed_notify ON %[1]q;
CREATE TRIGGER route_rows_changed_notify
  AFTER INSERT OR UPDATE OR DELETE ON %[1]q
  FOR EACH ROW EXECUTE FUNCTION route_rows_changed_notify('%[2]s');`, table, channel)
}

func Reconcile(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock(?)", reconcileLock); err != nil {
			return fmt.Errorf("take reconcile lock: %w", err)
		}
		if _, err := tx.ExecContext(ctx, liveDataSubscriptionsDDL); err != nil {
			return fmt.Errorf("reconcile live_data_subscriptions notify trigger: %w", err)
		}
		if _, err := tx.ExecContext(ctx, liveDataUnavailableDDL); err != nil {
			return fmt.Errorf("reconcile live_data_subscriptions unavailable trigger: %w", err)
		}
		if _, err := tx.ExecContext(ctx, liveDataAvailableDDL); err != nil {
			return fmt.Errorf("reconcile live_data_available notify trigger: %w", err)
		}
		if _, err := tx.ExecContext(ctx, routeRowsChangedFunctionDDL); err != nil {
			return fmt.Errorf("reconcile route rows changed function: %w", err)
		}
		for table, channel := range routeRowsChangedTables {
			if _, err := tx.ExecContext(ctx, routeRowsChangedTriggerDDL(table, channel)); err != nil {
				return fmt.Errorf("reconcile %s notify trigger: %w", table, err)
			}
		}
		return nil
	})
}

var routeRowsChangedTables = map[string]string{
	"vehicle":   VehiclesChannel,
	"departure": DeparturesChannel,
}
