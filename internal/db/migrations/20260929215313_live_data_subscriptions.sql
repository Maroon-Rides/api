-- +goose Up
-- create "live_data_subscriptions" table
CREATE TABLE "live_data_subscriptions" (
  "id" uuid NOT NULL DEFAULT uuidv7(),
  "routeId" uuid NOT NULL,
  "clientId" uuid NOT NULL,
  "lastUpdatedAt" timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY ("id"),
  CONSTRAINT "route_client_uq" UNIQUE ("routeId", "clientId"),
  CONSTRAINT "live_data_subscriptions_routeId_fkey" FOREIGN KEY ("routeId") REFERENCES "route" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);

-- +goose Down
-- reverse: create "live_data_subscriptions" table
DROP TABLE "live_data_subscriptions";
