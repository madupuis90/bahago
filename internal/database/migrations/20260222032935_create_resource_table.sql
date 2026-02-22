-- +goose Up
CREATE TABLE resources (
  id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  wood integer NOT NULL,
  stone integer NOT NULL,
  food integer NOT NULL
);
-- +goose Down
DROP TABLE IF EXISTS resources;