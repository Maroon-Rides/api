-- +goose Up
-- modify "vehicle" table
ALTER TABLE "vehicle" ADD COLUMN "passengers" bigint NOT NULL;

-- +goose Down
-- reverse: modify "vehicle" table
ALTER TABLE "vehicle" DROP COLUMN "passengers";
