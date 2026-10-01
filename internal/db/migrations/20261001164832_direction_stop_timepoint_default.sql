-- +goose Up
-- modify "direction_stop" table
ALTER TABLE "direction_stop" ALTER COLUMN "isTimepoint" SET DEFAULT false;

-- +goose Down
-- reverse: modify "direction_stop" table
ALTER TABLE "direction_stop" ALTER COLUMN "isTimepoint" DROP DEFAULT;
