-- +goose Up
-- modify "route" table
ALTER TABLE "route" ADD COLUMN "liveDataAvailable" boolean NOT NULL DEFAULT false;

-- +goose Down
-- reverse: modify "route" table
ALTER TABLE "route" DROP COLUMN "liveDataAvailable";
