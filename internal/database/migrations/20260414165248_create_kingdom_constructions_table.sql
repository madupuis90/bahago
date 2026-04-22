-- +goose Up
CREATE TABLE kingdom_constructions (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    kingdom_id      BIGINT NOT NULL REFERENCES kingdoms(id),
    building_type   TEXT   NOT NULL,
    ticks_remaining INT    NOT NULL,
    ticks_total     INT    NOT NULL,
    started_at      TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT kingdom_constructions_ticks_non_negative CHECK (ticks_remaining >= 0),
    CONSTRAINT kingdom_constructions_ticks_total_positive CHECK (ticks_total > 0)
);

CREATE INDEX ON kingdom_constructions (kingdom_id);

-- +goose Down
DROP TABLE IF EXISTS kingdom_constructions;
