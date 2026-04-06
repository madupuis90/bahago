-- +goose Up
CREATE TABLE kingdoms (
    id      BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id),
    name    TEXT NOT NULL,

    -- Population
    population BIGINT NOT NULL DEFAULT 100,

    -- Population allocation (must sum to 100; each value 0–100 enforced by constraint)
    wood_pct      INT NOT NULL DEFAULT 20 CHECK (wood_pct      BETWEEN 0 AND 100),
    stone_pct     INT NOT NULL DEFAULT 20 CHECK (stone_pct     BETWEEN 0 AND 100),
    food_pct      INT NOT NULL DEFAULT 20 CHECK (food_pct      BETWEEN 0 AND 100),
    mana_pct      INT NOT NULL DEFAULT 10 CHECK (mana_pct      BETWEEN 0 AND 100),
    devotion_pct  INT NOT NULL DEFAULT 10 CHECK (devotion_pct  BETWEEN 0 AND 100),
    knowledge_pct INT NOT NULL DEFAULT 10 CHECK (knowledge_pct BETWEEN 0 AND 100),
    idle_pct      INT NOT NULL DEFAULT 10 CHECK (idle_pct      BETWEEN 0 AND 100),

    -- Resources
    wood   BIGINT NOT NULL DEFAULT 0,
    stone  BIGINT NOT NULL DEFAULT 0,
    food      BIGINT NOT NULL DEFAULT 0,
    mana      BIGINT NOT NULL DEFAULT 0,
    devotion  BIGINT NOT NULL DEFAULT 0,
    knowledge BIGINT NOT NULL DEFAULT 0,

    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX kingdoms_user_id_idx ON kingdoms(user_id);

-- +goose Down
DROP TABLE IF EXISTS kingdoms;
