-- +goose Up
CREATE TABLE kingdom_prayers (
    id                BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    kingdom_id        BIGINT NOT NULL REFERENCES kingdoms(id),
    target_kingdom_id BIGINT NOT NULL REFERENCES kingdoms(id),
    prayer_type       TEXT NOT NULL,
    ticks_remaining   INT NOT NULL CHECK (ticks_remaining >= 0),
    ticks_total       INT NOT NULL,
    started_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT kingdom_prayers_unique_target_type UNIQUE (target_kingdom_id, prayer_type),
    CONSTRAINT kingdom_prayers_ticks_total_valid CHECK (ticks_total BETWEEN 1 AND 48)
);

CREATE INDEX ON kingdom_prayers (kingdom_id);
CREATE INDEX ON kingdom_prayers (target_kingdom_id);

-- +goose Down
DROP TABLE kingdom_prayers;
