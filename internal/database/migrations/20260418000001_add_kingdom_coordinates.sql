-- +goose Up
ALTER TABLE kingdoms
    ADD COLUMN x INT NOT NULL CHECK (x >= 0),
    ADD COLUMN y INT NOT NULL CHECK (y >= 0);

CREATE UNIQUE INDEX kingdoms_position_idx ON kingdoms(x, y);

-- +goose Down
DROP INDEX IF EXISTS kingdoms_position_idx;

ALTER TABLE kingdoms
    DROP COLUMN IF EXISTS x,
    DROP COLUMN IF EXISTS y;
