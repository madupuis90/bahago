-- +goose Up
CREATE TABLE kingdom_buildings (
    kingdom_id    BIGINT NOT NULL REFERENCES kingdoms(id),
    building_type TEXT   NOT NULL,
    count         INT    NOT NULL DEFAULT 0,

    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT kingdom_buildings_pkey PRIMARY KEY (kingdom_id, building_type),
    CONSTRAINT kingdom_buildings_count_non_negative CHECK (count >= 0)
);

-- +goose Down
DROP TABLE IF EXISTS kingdom_buildings;
