-- +goose Up
-- create "sync_metadata" table
CREATE TABLE "sync_metadata" (
  "key" character varying NOT NULL,
  "value" uuid NOT NULL,
  PRIMARY KEY ("key")
);

-- +goose Down
-- reverse: create "sync_metadata" table
DROP TABLE "sync_metadata";
