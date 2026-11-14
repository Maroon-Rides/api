-- +goose Up
-- modify "route" table
-- existing rows start as loops; the next route data sync marks the game day routes
ALTER TABLE "route" ADD COLUMN "isLoop" boolean NOT NULL DEFAULT true;
ALTER TABLE "route" ALTER COLUMN "isLoop" DROP DEFAULT;

-- +goose Down
-- the bump trigger compares isLoop; reconcile recreates it after migrating
DROP TRIGGER IF EXISTS route_sync_bump ON "route";
-- reverse: modify "route" table
ALTER TABLE "route" DROP COLUMN "isLoop";
