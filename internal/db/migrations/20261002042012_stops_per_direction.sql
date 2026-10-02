-- +goose Up
-- drop "direction_stop" table
DROP TABLE "direction_stop";
-- drop "direction_stop_audit" table
DROP TABLE "direction_stop_audit";
DROP FUNCTION IF EXISTS direction_stop_sync_tombstone();
-- stops have no direction to carry over; the next route data sync refills them
DELETE FROM "stop";
-- modify "departure" table
ALTER TABLE "departure" DROP CONSTRAINT "departure_slot_uq", DROP COLUMN "directionId", ADD CONSTRAINT "departure_slot_uq" UNIQUE ("stopId", "scheduledAt");
-- the bump trigger compares the dropped columns; reconcile recreates it after migrating
DROP TRIGGER IF EXISTS timetable_sync_bump ON "timetable";
-- modify "timetable" table
ALTER TABLE "timetable" DROP CONSTRAINT "timetable_slot_uq", DROP COLUMN "directionId", DROP COLUMN "routeId", ADD CONSTRAINT "timetable_slot_uq" UNIQUE ("stopId", "serviceDate");
-- modify "direction" table
ALTER TABLE "direction" DROP CONSTRAINT "direction_route_uq";
-- modify "stop" table
ALTER TABLE "stop" DROP CONSTRAINT "stop_sourceId_key", ADD COLUMN "directionId" uuid NOT NULL, ADD COLUMN "sequence" bigint NOT NULL, ADD COLUMN "isTimepoint" boolean NOT NULL DEFAULT false, ADD CONSTRAINT "stop_direction_source_uq" UNIQUE ("directionId", "sourceId"), ADD CONSTRAINT "stop_directionId_fkey" FOREIGN KEY ("directionId") REFERENCES "direction" ("id") ON UPDATE NO ACTION ON DELETE CASCADE;

-- +goose Down
-- reverse: drop "direction_stop_audit" table
CREATE TABLE "direction_stop_audit" (
  "id" uuid NOT NULL DEFAULT uuidv7(),
  "deletedAt" timestamptz NOT NULL DEFAULT clock_timestamp(),
  "directionStopId" uuid NOT NULL,
  PRIMARY KEY ("id")
);
CREATE INDEX "direction_stop_audit_deletedAt_idx" ON "direction_stop_audit" ("deletedAt");
-- reverse: drop "direction_stop" table
CREATE TABLE "direction_stop" (
  "updatedAt" timestamptz NOT NULL DEFAULT clock_timestamp(),
  "updateId" uuid NOT NULL DEFAULT uuidv7(),
  "id" uuid NOT NULL DEFAULT uuidv7(),
  "directionId" uuid NOT NULL,
  "stopId" uuid NOT NULL,
  "sequence" bigint NOT NULL,
  "isTimepoint" boolean NOT NULL DEFAULT false,
  PRIMARY KEY ("id"),
  CONSTRAINT "direction_stop_uq" UNIQUE ("directionId", "stopId"),
  CONSTRAINT "direction_stop_directionId_fkey" FOREIGN KEY ("directionId") REFERENCES "direction" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "direction_stop_stopId_fkey" FOREIGN KEY ("stopId") REFERENCES "stop" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
CREATE INDEX "direction_stop_updateId_idx" ON "direction_stop" ("updateId");
-- reverse: modify "stop" table
ALTER TABLE "stop" DROP CONSTRAINT "stop_directionId_fkey", DROP CONSTRAINT "stop_direction_source_uq", DROP COLUMN "isTimepoint", DROP COLUMN "sequence", DROP COLUMN "directionId", ADD CONSTRAINT "stop_sourceId_key" UNIQUE ("sourceId");
-- reverse: modify "timetable" table
ALTER TABLE "timetable" DROP CONSTRAINT "timetable_slot_uq", ADD COLUMN "routeId" uuid NOT NULL, ADD COLUMN "directionId" uuid NOT NULL, ADD CONSTRAINT "timetable_slot_uq" UNIQUE ("stopId", "directionId", "serviceDate");
-- reverse: modify "direction" table
ALTER TABLE "direction" ADD CONSTRAINT "direction_route_uq" UNIQUE ("id", "routeId");
-- reverse: modify "departure" table
ALTER TABLE "departure" DROP CONSTRAINT "departure_slot_uq", ADD COLUMN "directionId" uuid NOT NULL, ADD CONSTRAINT "departure_slot_uq" UNIQUE ("routeId", "stopId", "directionId", "scheduledAt");
