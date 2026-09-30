-- +goose Up
-- create "timetable_audit" table
CREATE TABLE "timetable_audit" (
  "id" uuid NOT NULL DEFAULT uuidv7(),
  "deletedAt" timestamptz NOT NULL DEFAULT clock_timestamp(),
  "timetableId" uuid NOT NULL,
  PRIMARY KEY ("id")
);
-- create index "timetable_audit_deletedAt_idx" to table: "timetable_audit"
CREATE INDEX "timetable_audit_deletedAt_idx" ON "timetable_audit" ("deletedAt");
-- create "timetable" table
CREATE TABLE "timetable" (
  "updatedAt" timestamptz NOT NULL DEFAULT clock_timestamp(),
  "updateId" uuid NOT NULL DEFAULT uuidv7(),
  "id" uuid NOT NULL DEFAULT uuidv7(),
  "stopId" uuid NOT NULL,
  "directionId" uuid NOT NULL,
  "serviceDate" date NOT NULL,
  "departures" timestamp with time zone[] NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "timetable_slot_uq" UNIQUE ("stopId", "directionId", "serviceDate"),
  CONSTRAINT "timetable_directionId_fkey" FOREIGN KEY ("directionId") REFERENCES "direction" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "timetable_stopId_fkey" FOREIGN KEY ("stopId") REFERENCES "stop" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- create index "timetable_updateId_idx" to table: "timetable"
CREATE INDEX "timetable_updateId_idx" ON "timetable" ("updateId");
-- drop "stop_schedule" table
DROP TABLE "stop_schedule";
-- drop "stop_schedule_audit" table
DROP TABLE "stop_schedule_audit";

-- +goose Down
-- reverse: drop "stop_schedule_audit" table
CREATE TABLE "stop_schedule_audit" (
  "id" uuid NOT NULL DEFAULT uuidv7(),
  "deletedAt" timestamptz NOT NULL DEFAULT clock_timestamp(),
  "stopScheduleId" uuid NOT NULL,
  PRIMARY KEY ("id")
);
CREATE INDEX "stop_schedule_audit_deletedAt_idx" ON "stop_schedule_audit" ("deletedAt");
-- reverse: drop "stop_schedule" table
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
CREATE INDEX "stop_schedule_updateId_idx" ON "stop_schedule" ("updateId");
-- reverse: create index "timetable_updateId_idx" to table: "timetable"
DROP INDEX "timetable_updateId_idx";
-- reverse: create "timetable" table
DROP TABLE "timetable";
-- reverse: create index "timetable_audit_deletedAt_idx" to table: "timetable_audit"
DROP INDEX "timetable_audit_deletedAt_idx";
-- reverse: create "timetable_audit" table
DROP TABLE "timetable_audit";
