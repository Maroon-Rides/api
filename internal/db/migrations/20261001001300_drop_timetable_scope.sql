-- +goose Up
-- drop index "timetable_routeId_updateId_idx" from table: "timetable"
DROP INDEX "timetable_routeId_updateId_idx";
-- modify "timetable_audit" table
ALTER TABLE "timetable_audit" DROP COLUMN "routeId";
-- atlas does not track triggers; the reconcile after migrations stops writing "routeId" to tombstones
DROP TRIGGER IF EXISTS timetable_sync_freeze_scope ON "timetable";
DROP FUNCTION IF EXISTS timetable_sync_freeze_scope();

-- +goose Down
-- reverse: modify "timetable_audit" table
ALTER TABLE "timetable_audit" ADD COLUMN "routeId" uuid NOT NULL;
-- reverse: drop index "timetable_routeId_updateId_idx" from table: "timetable"
CREATE INDEX "timetable_routeId_updateId_idx" ON "timetable" ("routeId", "updateId");
