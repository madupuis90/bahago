-- +goose Up
CREATE TABLE kingdom_training (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    kingdom_id      BIGINT NOT NULL REFERENCES kingdoms(id),
    unit_type       TEXT   NOT NULL,
    count           INT    NOT NULL,
    ticks_remaining INT    NOT NULL,
    ticks_total     INT    NOT NULL,
    started_at      TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT kingdom_training_count_positive CHECK (count > 0),
    CONSTRAINT kingdom_training_ticks_non_negative CHECK (ticks_remaining >= 0),
    CONSTRAINT kingdom_training_ticks_total_positive CHECK (ticks_total > 0)
);

CREATE INDEX ON kingdom_training (kingdom_id);

-- +goose Down
DROP TABLE IF EXISTS kingdom_training;
