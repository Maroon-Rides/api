-- +goose Up
-- modify "direction" table
ALTER TABLE "direction" ADD CONSTRAINT "direction_route_destination_uq" UNIQUE ("routeId", "destination");
-- modify "route" table
ALTER TABLE "route" ADD CONSTRAINT "route_shortName_key" UNIQUE ("shortName");

-- +goose Down
-- reverse: modify "route" table
ALTER TABLE "route" DROP CONSTRAINT "route_shortName_key";
-- reverse: modify "direction" table
ALTER TABLE "direction" DROP CONSTRAINT "direction_route_destination_uq";
