-- +goose Up
-- modify "direction" table
ALTER TABLE "direction" ADD CONSTRAINT "direction_route_uq" UNIQUE ("id", "routeId");
-- modify "timetable_audit" table
-- no client holds a per-route timetable ack yet, so no older tombstone can ever be sent
DELETE FROM "timetable_audit";
ALTER TABLE "timetable_audit" ADD COLUMN "routeId" uuid NOT NULL;
-- create index "timetable_audit_routeId_id_idx" to table: "timetable_audit"
CREATE INDEX "timetable_audit_routeId_id_idx" ON "timetable_audit" ("routeId", "id");
-- modify "timetable" table
ALTER TABLE "timetable" ADD COLUMN "routeId" uuid;
UPDATE "timetable" AS t SET "routeId" = d."routeId" FROM "direction" AS d WHERE d."id" = t."directionId";
ALTER TABLE "timetable" DROP CONSTRAINT "timetable_directionId_fkey", ALTER COLUMN "routeId" SET NOT NULL, ADD CONSTRAINT "timetable_directionId_routeId_fkey" FOREIGN KEY ("directionId", "routeId") REFERENCES "direction" ("id", "routeId") ON UPDATE NO ACTION ON DELETE CASCADE;
-- create index "timetable_routeId_updateId_idx" to table: "timetable"
CREATE INDEX "timetable_routeId_updateId_idx" ON "timetable" ("routeId", "updateId");

-- +goose Down
-- reverse: create index "timetable_routeId_updateId_idx" to table: "timetable"
DROP INDEX "timetable_routeId_updateId_idx";
-- reverse: modify "timetable" table
ALTER TABLE "timetable" DROP CONSTRAINT "timetable_directionId_routeId_fkey", DROP COLUMN "routeId", ADD CONSTRAINT "timetable_directionId_fkey" FOREIGN KEY ("directionId") REFERENCES "direction" ("id") ON UPDATE NO ACTION ON DELETE CASCADE;
-- reverse: create index "timetable_audit_routeId_id_idx" to table: "timetable_audit"
DROP INDEX "timetable_audit_routeId_id_idx";
-- reverse: modify "timetable_audit" table
ALTER TABLE "timetable_audit" DROP COLUMN "routeId";
-- reverse: modify "direction" table
ALTER TABLE "direction" DROP CONSTRAINT "direction_route_uq";
