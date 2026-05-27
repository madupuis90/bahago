-- +goose Up

-- Drop old view and tables (greenfield: no data to migrate).
-- kingdom_campaign_units is dropped first in case a prior partial run left it behind.
DROP VIEW  IF EXISTS kingdom_available_units;
DROP TABLE IF EXISTS kingdom_campaign_units;
DROP TABLE IF EXISTS kingdom_campaigns;

-- Persistent named unit containers per Kingdom
CREATE TABLE kingdom_legions (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    kingdom_id BIGINT NOT NULL REFERENCES kingdoms(id) ON DELETE CASCADE,
    number     INT    NOT NULL CHECK (number > 0),
    name       TEXT   NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT kingdom_legions_unique_number UNIQUE (kingdom_id, number)
);
CREATE INDEX idx_kingdom_legions_kingdom ON kingdom_legions(kingdom_id);

-- Units held inside a Legion (zero-count rows are deleted, never stored)
CREATE TABLE kingdom_legion_units (
    legion_id BIGINT NOT NULL REFERENCES kingdom_legions(id) ON DELETE CASCADE,
    unit_type TEXT   NOT NULL,
    count     INT    NOT NULL CHECK (count > 0),
    PRIMARY KEY (legion_id, unit_type)
);

-- Campaigns now reference a Legion instead of a single unit_type + count
CREATE TABLE kingdom_campaigns (
    id                BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    kingdom_id        BIGINT NOT NULL REFERENCES kingdoms(id),
    target_kingdom_id BIGINT NOT NULL REFERENCES kingdoms(id),
    legion_id         BIGINT NOT NULL REFERENCES kingdom_legions(id),
    action            TEXT   NOT NULL,
    status            TEXT   NOT NULL DEFAULT 'en_route',
    ticks_remaining   INT    NOT NULL CHECK (ticks_remaining >= 0),
    action_ticks      INT    NOT NULL CHECK (action_ticks > 0),
    travel_ticks      INT    NOT NULL CHECK (travel_ticks >= 3),
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT kingdom_campaigns_action_valid   CHECK (action IN ('attack', 'defend')),
    CONSTRAINT kingdom_campaigns_status_valid   CHECK (status IN ('en_route', 'active', 'returning')),
    CONSTRAINT kingdom_campaigns_no_self_target CHECK (kingdom_id != target_kingdom_id),
    CONSTRAINT kingdom_campaigns_one_per_legion UNIQUE (legion_id)
);
CREATE INDEX idx_kingdom_campaigns_kingdom ON kingdom_campaigns(kingdom_id);
CREATE INDEX idx_kingdom_campaigns_target  ON kingdom_campaigns(target_kingdom_id);
CREATE INDEX idx_kingdom_campaigns_legion  ON kingdom_campaigns(legion_id);
-- Tick query: GetActiveCampaignsReadyForCombat filters on status + ticks_remaining every tick.
CREATE INDEX idx_kingdom_campaigns_tick    ON kingdom_campaigns(ticks_remaining) WHERE status = 'active';

-- Snapshot of Legion composition taken at campaign departure
CREATE TABLE kingdom_campaign_units (
    campaign_id BIGINT NOT NULL REFERENCES kingdom_campaigns(id) ON DELETE CASCADE,
    unit_type   TEXT   NOT NULL,
    count       INT    NOT NULL CHECK (count > 0),
    PRIMARY KEY (campaign_id, unit_type)
);

-- Reserve = kingdom_units minus Legion assignments minus campaign snapshots
CREATE VIEW kingdom_available_units AS
SELECT
    ku.kingdom_id,
    ku.unit_type,
    (
        ku.count
        - COALESCE((SELECT SUM(klu.count)
                    FROM kingdom_legion_units klu
                    JOIN kingdom_legions kl ON kl.id = klu.legion_id
                    WHERE kl.kingdom_id = ku.kingdom_id AND klu.unit_type = ku.unit_type), 0)
        - COALESCE((SELECT SUM(kcu.count)
                    FROM kingdom_campaign_units kcu
                    JOIN kingdom_campaigns kc ON kc.id = kcu.campaign_id
                    WHERE kc.kingdom_id = ku.kingdom_id AND kcu.unit_type = ku.unit_type), 0)
    )::int AS count
FROM kingdom_units ku;

-- +goose Down

DROP VIEW  IF EXISTS kingdom_available_units;
DROP TABLE IF EXISTS kingdom_campaign_units;
DROP TABLE IF EXISTS kingdom_campaigns;
DROP TABLE IF EXISTS kingdom_legion_units;
DROP TABLE IF EXISTS kingdom_legions;

-- Restore original kingdom_campaigns table
CREATE TABLE kingdom_campaigns (
    id                BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    kingdom_id        BIGINT NOT NULL REFERENCES kingdoms(id),
    target_kingdom_id BIGINT NOT NULL REFERENCES kingdoms(id),
    unit_type         TEXT NOT NULL,
    count             INT NOT NULL CHECK (count >= 0),
    action            TEXT NOT NULL,
    status            TEXT NOT NULL DEFAULT 'en_route',
    ticks_remaining   INT NOT NULL CHECK (ticks_remaining >= 0),
    action_ticks      INT NOT NULL CHECK (action_ticks > 0),
    travel_ticks      INT NOT NULL CHECK (travel_ticks >= 3),
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT kingdom_campaigns_action_valid   CHECK (action IN ('attack', 'defend')),
    CONSTRAINT kingdom_campaigns_status_valid   CHECK (status IN ('en_route', 'active', 'returning')),
    CONSTRAINT kingdom_campaigns_no_self_target CHECK (kingdom_id != target_kingdom_id)
);
CREATE INDEX ON kingdom_campaigns (kingdom_id);
CREATE INDEX ON kingdom_campaigns (target_kingdom_id);
CREATE INDEX ON kingdom_campaigns (ticks_remaining) WHERE status = 'active';

-- Restore original kingdom_available_units view
CREATE VIEW kingdom_available_units AS
SELECT
    ku.kingdom_id,
    ku.unit_type,
    (ku.count - COALESCE(SUM(kc.count), 0))::int AS count
FROM kingdom_units ku
LEFT JOIN kingdom_campaigns kc
    ON kc.kingdom_id = ku.kingdom_id
    AND kc.unit_type = ku.unit_type
GROUP BY ku.kingdom_id, ku.unit_type;
