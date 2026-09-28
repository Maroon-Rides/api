-- +goose Up
-- create "stop_schedule_audit" table
CREATE TABLE "stop_schedule_audit" (
  "id" uuid NOT NULL DEFAULT uuidv7(),
  "deletedAt" timestamptz NOT NULL DEFAULT clock_timestamp(),
  "stopScheduleId" uuid NOT NULL,
  PRIMARY KEY ("id")
);
-- create index "stop_schedule_audit_deletedAt_idx" to table: "stop_schedule_audit"
CREATE INDEX "stop_schedule_audit_deletedAt_idx" ON "stop_schedule_audit" ("deletedAt");
-- create "stop_schedule" table
CREATE TABLE "stop_schedule" (
  "updatedAt" timestamptz NOT NULL DEFAULT clock_timestamp(),
  "updateId" uuid NOT NULL DEFAULT uuidv7(),
  "id" uuid NOT NULL DEFAULT uuidv7(),
  "stopId" uuid NOT NULL,
  "directionId" uuid NOT NULL,
  "scheduledAt" timestamptz NOT NULL,
  "serviceDate" date NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "stop_schedule_slot_uq" UNIQUE ("stopId", "directionId", "scheduledAt"),
  CONSTRAINT "stop_schedule_directionId_fkey" FOREIGN KEY ("directionId") REFERENCES "direction" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "stop_schedule_stopId_fkey" FOREIGN KEY ("stopId") REFERENCES "stop" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- create index "stop_schedule_updateId_idx" to table: "stop_schedule"
CREATE INDEX "stop_schedule_updateId_idx" ON "stop_schedule" ("updateId");
-- drop "trip_audit" table
DROP TABLE "trip_audit";
-- drop "trip_stop_time_audit" table
DROP TABLE "trip_stop_time_audit";
-- drop "trip_stop_time" table
DROP TABLE "trip_stop_time";
-- drop "trip" table
DROP TABLE "trip";

-- +goose Down
-- reverse: drop "trip" table
CREATE TABLE "trip" (
  "updatedAt" timestamptz NOT NULL DEFAULT clock_timestamp(),
  "updateId" uuid NOT NULL DEFAULT uuidv7(),
  "id" uuid NOT NULL DEFAULT uuidv7(),
  "directionId" uuid NOT NULL,
  "serviceDate" date NOT NULL,
  "startsAt" timestamptz NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "trip_slot_uq" UNIQUE ("directionId", "serviceDate", "startsAt"),
  CONSTRAINT "trip_directionId_fkey" FOREIGN KEY ("directionId") REFERENCES "direction" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
CREATE INDEX "trip_updateId_idx" ON "trip" ("updateId");
-- reverse: drop "trip_stop_time" table
CREATE TABLE "trip_stop_time" (
  "updatedAt" timestamptz NOT NULL DEFAULT clock_timestamp(),
  "updateId" uuid NOT NULL DEFAULT uuidv7(),
  "id" uuid NOT NULL DEFAULT uuidv7(),
  "tripId" uuid NOT NULL,
  "stopId" uuid NOT NULL,
  "sequence" bigint NOT NULL,
  "scheduledAt" timestamptz NOT NULL,
  "isLastPoint" boolean NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "trip_stop_uq" UNIQUE ("tripId", "stopId"),
  CONSTRAINT "trip_stop_time_stopId_fkey" FOREIGN KEY ("stopId") REFERENCES "stop" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "trip_stop_time_tripId_fkey" FOREIGN KEY ("tripId") REFERENCES "trip" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
CREATE INDEX "trip_stop_time_updateId_idx" ON "trip_stop_time" ("updateId");
-- reverse: drop "trip_stop_time_audit" table
CREATE TABLE "trip_stop_time_audit" (
  "id" uuid NOT NULL DEFAULT uuidv7(),
  "deletedAt" timestamptz NOT NULL DEFAULT clock_timestamp(),
  "tripStopTimeId" uuid NOT NULL,
  PRIMARY KEY ("id")
);
CREATE INDEX "trip_stop_time_audit_deletedAt_idx" ON "trip_stop_time_audit" ("deletedAt");
-- reverse: drop "trip_audit" table
CREATE TABLE "trip_audit" (
  "id" uuid NOT NULL DEFAULT uuidv7(),
  "deletedAt" timestamptz NOT NULL DEFAULT clock_timestamp(),
  "tripId" uuid NOT NULL,
  PRIMARY KEY ("id")
);
CREATE INDEX "trip_audit_deletedAt_idx" ON "trip_audit" ("deletedAt");
-- reverse: create index "stop_schedule_updateId_idx" to table: "stop_schedule"
DROP INDEX "stop_schedule_updateId_idx";
-- reverse: create "stop_schedule" table
DROP TABLE "stop_schedule";
-- reverse: create index "stop_schedule_audit_deletedAt_idx" to table: "stop_schedule_audit"
DROP INDEX "stop_schedule_audit_deletedAt_idx";
-- reverse: create "stop_schedule_audit" table
DROP TABLE "stop_schedule_audit";
