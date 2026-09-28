-- +goose Up
-- create "alert" table
CREATE TABLE "alert" (
  "updatedAt" timestamptz NOT NULL DEFAULT clock_timestamp(),
  "updateId" uuid NOT NULL DEFAULT uuidv7(),
  "id" uuid NOT NULL DEFAULT uuidv7(),
  "sourceId" character varying NOT NULL,
  "title" character varying NOT NULL,
  "description" character varying NOT NULL,
  "timeRangeText" character varying NOT NULL,
  "dailyStartTime" character varying NOT NULL,
  "dailyEndTime" character varying NOT NULL,
  "startsAt" timestamptz NOT NULL,
  "endsAt" timestamptz NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "alert_sourceId_key" UNIQUE ("sourceId")
);
-- create index "alert_updateId_idx" to table: "alert"
CREATE INDEX "alert_updateId_idx" ON "alert" ("updateId");
-- create "alert_audit" table
CREATE TABLE "alert_audit" (
  "id" uuid NOT NULL DEFAULT uuidv7(),
  "deletedAt" timestamptz NOT NULL DEFAULT clock_timestamp(),
  "alertId" uuid NOT NULL,
  PRIMARY KEY ("id")
);
-- create index "alert_audit_deletedAt_idx" to table: "alert_audit"
CREATE INDEX "alert_audit_deletedAt_idx" ON "alert_audit" ("deletedAt");
-- create "alert_direction_audit" table
CREATE TABLE "alert_direction_audit" (
  "id" uuid NOT NULL DEFAULT uuidv7(),
  "deletedAt" timestamptz NOT NULL DEFAULT clock_timestamp(),
  "alertDirectionId" uuid NOT NULL,
  PRIMARY KEY ("id")
);
-- create index "alert_direction_audit_deletedAt_idx" to table: "alert_direction_audit"
CREATE INDEX "alert_direction_audit_deletedAt_idx" ON "alert_direction_audit" ("deletedAt");
-- create "direction_audit" table
CREATE TABLE "direction_audit" (
  "id" uuid NOT NULL DEFAULT uuidv7(),
  "deletedAt" timestamptz NOT NULL DEFAULT clock_timestamp(),
  "directionId" uuid NOT NULL,
  PRIMARY KEY ("id")
);
-- create index "direction_audit_deletedAt_idx" to table: "direction_audit"
CREATE INDEX "direction_audit_deletedAt_idx" ON "direction_audit" ("deletedAt");
-- create "direction_stop_audit" table
CREATE TABLE "direction_stop_audit" (
  "id" uuid NOT NULL DEFAULT uuidv7(),
  "deletedAt" timestamptz NOT NULL DEFAULT clock_timestamp(),
  "directionStopId" uuid NOT NULL,
  PRIMARY KEY ("id")
);
-- create index "direction_stop_audit_deletedAt_idx" to table: "direction_stop_audit"
CREATE INDEX "direction_stop_audit_deletedAt_idx" ON "direction_stop_audit" ("deletedAt");
-- create "route_audit" table
CREATE TABLE "route_audit" (
  "id" uuid NOT NULL DEFAULT uuidv7(),
  "deletedAt" timestamptz NOT NULL DEFAULT clock_timestamp(),
  "routeId" uuid NOT NULL,
  PRIMARY KEY ("id")
);
-- create index "route_audit_deletedAt_idx" to table: "route_audit"
CREATE INDEX "route_audit_deletedAt_idx" ON "route_audit" ("deletedAt");
-- create "stop_audit" table
CREATE TABLE "stop_audit" (
  "id" uuid NOT NULL DEFAULT uuidv7(),
  "deletedAt" timestamptz NOT NULL DEFAULT clock_timestamp(),
  "stopId" uuid NOT NULL,
  PRIMARY KEY ("id")
);
-- create index "stop_audit_deletedAt_idx" to table: "stop_audit"
CREATE INDEX "stop_audit_deletedAt_idx" ON "stop_audit" ("deletedAt");
-- create "trip_audit" table
CREATE TABLE "trip_audit" (
  "id" uuid NOT NULL DEFAULT uuidv7(),
  "deletedAt" timestamptz NOT NULL DEFAULT clock_timestamp(),
  "tripId" uuid NOT NULL,
  PRIMARY KEY ("id")
);
-- create index "trip_audit_deletedAt_idx" to table: "trip_audit"
CREATE INDEX "trip_audit_deletedAt_idx" ON "trip_audit" ("deletedAt");
-- create "trip_stop_time_audit" table
CREATE TABLE "trip_stop_time_audit" (
  "id" uuid NOT NULL DEFAULT uuidv7(),
  "deletedAt" timestamptz NOT NULL DEFAULT clock_timestamp(),
  "tripStopTimeId" uuid NOT NULL,
  PRIMARY KEY ("id")
);
-- create index "trip_stop_time_audit_deletedAt_idx" to table: "trip_stop_time_audit"
CREATE INDEX "trip_stop_time_audit_deletedAt_idx" ON "trip_stop_time_audit" ("deletedAt");
-- create "route" table
CREATE TABLE "route" (
  "updatedAt" timestamptz NOT NULL DEFAULT clock_timestamp(),
  "updateId" uuid NOT NULL DEFAULT uuidv7(),
  "id" uuid NOT NULL DEFAULT uuidv7(),
  "sourceId" character varying NOT NULL,
  "shortName" character varying NOT NULL,
  "longName" character varying NOT NULL,
  "lightColor" character varying NOT NULL,
  "darkColor" character varying NOT NULL,
  "active" boolean NOT NULL DEFAULT true,
  "deactivatedAt" timestamptz NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "route_sourceId_key" UNIQUE ("sourceId")
);
-- create index "route_updateId_idx" to table: "route"
CREATE INDEX "route_updateId_idx" ON "route" ("updateId");
-- create "direction" table
CREATE TABLE "direction" (
  "updatedAt" timestamptz NOT NULL DEFAULT clock_timestamp(),
  "updateId" uuid NOT NULL DEFAULT uuidv7(),
  "id" uuid NOT NULL DEFAULT uuidv7(),
  "routeId" uuid NOT NULL,
  "sourceId" character varying NOT NULL,
  "destination" character varying NOT NULL,
  "sequence" bigint NOT NULL,
  "path" character varying NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "direction_sourceId_key" UNIQUE ("sourceId"),
  CONSTRAINT "direction_routeId_fkey" FOREIGN KEY ("routeId") REFERENCES "route" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- create index "direction_updateId_idx" to table: "direction"
CREATE INDEX "direction_updateId_idx" ON "direction" ("updateId");
-- create "alert_direction" table
CREATE TABLE "alert_direction" (
  "updatedAt" timestamptz NOT NULL DEFAULT clock_timestamp(),
  "updateId" uuid NOT NULL DEFAULT uuidv7(),
  "id" uuid NOT NULL DEFAULT uuidv7(),
  "alertId" uuid NOT NULL,
  "directionId" uuid NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "alert_direction_uq" UNIQUE ("alertId", "directionId"),
  CONSTRAINT "alert_direction_alertId_fkey" FOREIGN KEY ("alertId") REFERENCES "alert" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "alert_direction_directionId_fkey" FOREIGN KEY ("directionId") REFERENCES "direction" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- create index "alert_direction_updateId_idx" to table: "alert_direction"
CREATE INDEX "alert_direction_updateId_idx" ON "alert_direction" ("updateId");
-- create "stop" table
CREATE TABLE "stop" (
  "updatedAt" timestamptz NOT NULL DEFAULT clock_timestamp(),
  "updateId" uuid NOT NULL DEFAULT uuidv7(),
  "id" uuid NOT NULL DEFAULT uuidv7(),
  "sourceId" character varying NOT NULL,
  "name" character varying NOT NULL,
  "lat" double precision NOT NULL,
  "lon" double precision NOT NULL,
  "amenities" jsonb NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "stop_sourceId_key" UNIQUE ("sourceId")
);
-- create index "stop_updateId_idx" to table: "stop"
CREATE INDEX "stop_updateId_idx" ON "stop" ("updateId");
-- create "departure" table
CREATE TABLE "departure" (
  "id" uuid NOT NULL DEFAULT uuidv7(),
  "routeId" uuid NOT NULL,
  "stopId" uuid NOT NULL,
  "directionId" uuid NOT NULL,
  "scheduledAt" timestamptz NOT NULL,
  "estimatedAt" timestamptz NULL,
  "isCancelled" boolean NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "departure_slot_uq" UNIQUE ("routeId", "stopId", "directionId", "scheduledAt"),
  CONSTRAINT "departure_directionId_fkey" FOREIGN KEY ("directionId") REFERENCES "direction" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "departure_routeId_fkey" FOREIGN KEY ("routeId") REFERENCES "route" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "departure_stopId_fkey" FOREIGN KEY ("stopId") REFERENCES "stop" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- create "direction_stop" table
CREATE TABLE "direction_stop" (
  "updatedAt" timestamptz NOT NULL DEFAULT clock_timestamp(),
  "updateId" uuid NOT NULL DEFAULT uuidv7(),
  "id" uuid NOT NULL DEFAULT uuidv7(),
  "directionId" uuid NOT NULL,
  "stopId" uuid NOT NULL,
  "sequence" bigint NOT NULL,
  "isTimepoint" boolean NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "direction_stop_uq" UNIQUE ("directionId", "stopId"),
  CONSTRAINT "direction_stop_directionId_fkey" FOREIGN KEY ("directionId") REFERENCES "direction" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "direction_stop_stopId_fkey" FOREIGN KEY ("stopId") REFERENCES "stop" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- create index "direction_stop_updateId_idx" to table: "direction_stop"
CREATE INDEX "direction_stop_updateId_idx" ON "direction_stop" ("updateId");
-- create "trip" table
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
-- create index "trip_updateId_idx" to table: "trip"
CREATE INDEX "trip_updateId_idx" ON "trip" ("updateId");
-- create "trip_stop_time" table
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
-- create index "trip_stop_time_updateId_idx" to table: "trip_stop_time"
CREATE INDEX "trip_stop_time_updateId_idx" ON "trip_stop_time" ("updateId");
-- create "vehicle" table
CREATE TABLE "vehicle" (
  "id" uuid NOT NULL DEFAULT uuidv7(),
  "sourceId" character varying NOT NULL,
  "routeId" uuid NOT NULL,
  "directionId" uuid NOT NULL,
  "name" character varying NOT NULL,
  "lat" double precision NOT NULL,
  "lon" double precision NOT NULL,
  "heading" double precision NOT NULL,
  "speed" double precision NOT NULL,
  "capacity" bigint NOT NULL,
  "amenities" jsonb NOT NULL,
  "seenAt" timestamptz NOT NULL DEFAULT clock_timestamp(),
  PRIMARY KEY ("id"),
  CONSTRAINT "vehicle_sourceId_key" UNIQUE ("sourceId"),
  CONSTRAINT "vehicle_directionId_fkey" FOREIGN KEY ("directionId") REFERENCES "direction" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "vehicle_routeId_fkey" FOREIGN KEY ("routeId") REFERENCES "route" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);

-- +goose Down
-- reverse: create "vehicle" table
DROP TABLE "vehicle";
-- reverse: create index "trip_stop_time_updateId_idx" to table: "trip_stop_time"
DROP INDEX "trip_stop_time_updateId_idx";
-- reverse: create "trip_stop_time" table
DROP TABLE "trip_stop_time";
-- reverse: create index "trip_updateId_idx" to table: "trip"
DROP INDEX "trip_updateId_idx";
-- reverse: create "trip" table
DROP TABLE "trip";
-- reverse: create index "direction_stop_updateId_idx" to table: "direction_stop"
DROP INDEX "direction_stop_updateId_idx";
-- reverse: create "direction_stop" table
DROP TABLE "direction_stop";
-- reverse: create "departure" table
DROP TABLE "departure";
-- reverse: create index "stop_updateId_idx" to table: "stop"
DROP INDEX "stop_updateId_idx";
-- reverse: create "stop" table
DROP TABLE "stop";
-- reverse: create index "alert_direction_updateId_idx" to table: "alert_direction"
DROP INDEX "alert_direction_updateId_idx";
-- reverse: create "alert_direction" table
DROP TABLE "alert_direction";
-- reverse: create index "direction_updateId_idx" to table: "direction"
DROP INDEX "direction_updateId_idx";
-- reverse: create "direction" table
DROP TABLE "direction";
-- reverse: create index "route_updateId_idx" to table: "route"
DROP INDEX "route_updateId_idx";
-- reverse: create "route" table
DROP TABLE "route";
-- reverse: create index "trip_stop_time_audit_deletedAt_idx" to table: "trip_stop_time_audit"
DROP INDEX "trip_stop_time_audit_deletedAt_idx";
-- reverse: create "trip_stop_time_audit" table
DROP TABLE "trip_stop_time_audit";
-- reverse: create index "trip_audit_deletedAt_idx" to table: "trip_audit"
DROP INDEX "trip_audit_deletedAt_idx";
-- reverse: create "trip_audit" table
DROP TABLE "trip_audit";
-- reverse: create index "stop_audit_deletedAt_idx" to table: "stop_audit"
DROP INDEX "stop_audit_deletedAt_idx";
-- reverse: create "stop_audit" table
DROP TABLE "stop_audit";
-- reverse: create index "route_audit_deletedAt_idx" to table: "route_audit"
DROP INDEX "route_audit_deletedAt_idx";
-- reverse: create "route_audit" table
DROP TABLE "route_audit";
-- reverse: create index "direction_stop_audit_deletedAt_idx" to table: "direction_stop_audit"
DROP INDEX "direction_stop_audit_deletedAt_idx";
-- reverse: create "direction_stop_audit" table
DROP TABLE "direction_stop_audit";
-- reverse: create index "direction_audit_deletedAt_idx" to table: "direction_audit"
DROP INDEX "direction_audit_deletedAt_idx";
-- reverse: create "direction_audit" table
DROP TABLE "direction_audit";
-- reverse: create index "alert_direction_audit_deletedAt_idx" to table: "alert_direction_audit"
DROP INDEX "alert_direction_audit_deletedAt_idx";
-- reverse: create "alert_direction_audit" table
DROP TABLE "alert_direction_audit";
-- reverse: create index "alert_audit_deletedAt_idx" to table: "alert_audit"
DROP INDEX "alert_audit_deletedAt_idx";
-- reverse: create "alert_audit" table
DROP TABLE "alert_audit";
-- reverse: create index "alert_updateId_idx" to table: "alert"
DROP INDEX "alert_updateId_idx";
-- reverse: create "alert" table
DROP TABLE "alert";
