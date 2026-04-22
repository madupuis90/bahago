-- +goose Up
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
    travel_ticks      INT NOT NULL CHECK (travel_ticks >= 12),
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT kingdom_campaigns_action_valid CHECK (action IN ('attack', 'defend')),
    CONSTRAINT kingdom_campaigns_status_valid CHECK (status IN ('en_route', 'active', 'returning')),
    CONSTRAINT kingdom_campaigns_no_self_target CHECK (kingdom_id != target_kingdom_id)
);

CREATE INDEX ON kingdom_campaigns (kingdom_id);
CREATE INDEX ON kingdom_campaigns (target_kingdom_id);
-- Tick query: GetActiveCampaignsReadyForCombat filters on status + ticks_remaining every tick.
CREATE INDEX ON kingdom_campaigns (ticks_remaining) WHERE status = 'active';

-- +goose Down
DROP TABLE kingdom_campaigns;
