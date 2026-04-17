-- +goose Up
CREATE TABLE kingdom_units (
    kingdom_id BIGINT NOT NULL REFERENCES kingdoms(id),
    unit_type  TEXT   NOT NULL,
    count      INT    NOT NULL DEFAULT 0,

    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT kingdom_units_pkey PRIMARY KEY (kingdom_id, unit_type),
    CONSTRAINT kingdom_units_count_non_negative CHECK (count >= 0)
);

-- +goose Down
DROP TABLE IF EXISTS kingdom_units;
