-- +goose Up
CREATE TABLE kingdom_combat_log (
    id                  BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tick_id             BIGINT NOT NULL REFERENCES game_ticks(id),
    target_kingdom_id   BIGINT NOT NULL REFERENCES kingdoms(id) ON DELETE CASCADE,
    -- Each element: {unit_type, count, power, casualties} — aggregated across all kingdoms on that side.
    attacker_units      JSONB NOT NULL,
    defender_units      JSONB NOT NULL,
    attacker_power      INT NOT NULL,
    defender_power      INT NOT NULL,
    winner              TEXT NOT NULL,
    attacker_casualties INT NOT NULL,
    defender_casualties INT NOT NULL,
    population_stolen   INT NOT NULL DEFAULT 0,
    occurred_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT combat_log_winner_valid CHECK (winner IN ('attacker', 'defender'))
);

-- Links each participating kingdom to a shared combat log entry.
CREATE TABLE kingdom_combat_log_participants (
    combat_log_id     BIGINT NOT NULL REFERENCES kingdom_combat_log(id) ON DELETE CASCADE,
    kingdom_id        BIGINT NOT NULL REFERENCES kingdoms(id) ON DELETE CASCADE,
    role              TEXT NOT NULL,
    population_gained INT NOT NULL DEFAULT 0,

    PRIMARY KEY (combat_log_id, kingdom_id),
    CONSTRAINT combat_log_participants_role_valid CHECK (role IN ('attacker', 'defender'))
);

-- Primary access pattern: recent combat at a given kingdom (target).
CREATE INDEX ON kingdom_combat_log (target_kingdom_id, occurred_at DESC);
-- Allows any participant to look up their combat history.
CREATE INDEX ON kingdom_combat_log_participants (kingdom_id);

-- +goose Down
DROP TABLE IF EXISTS kingdom_combat_log_participants;
DROP TABLE IF EXISTS kingdom_combat_log;
