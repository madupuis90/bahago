-- +goose Up
CREATE TABLE guild_memberships (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    guild_id   BIGINT NOT NULL REFERENCES guilds(id) ON DELETE CASCADE,
    kingdom_id BIGINT NOT NULL REFERENCES kingdoms(id) ON DELETE CASCADE,
    role      TEXT NOT NULL,
    joined_at  TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT guild_memberships_role_valid CHECK (role IN ('applicant', 'supporter', 'pending_approval', 'member', 'officer', 'leader'))
);

CREATE INDEX ON guild_memberships (guild_id);
CREATE INDEX ON guild_memberships (kingdom_id);

-- One active guild commitment per kingdom (excludes pending_approval which can be multiple)
CREATE UNIQUE INDEX guild_memberships_one_active_per_kingdom
    ON guild_memberships (kingdom_id)
    WHERE role IN ('applicant', 'supporter', 'member', 'officer', 'leader');

-- One pending request per kingdom per guild (prevents duplicate applications)
CREATE UNIQUE INDEX guild_memberships_one_pending_per_kingdom_per_guild
    ON guild_memberships (kingdom_id, guild_id)
    WHERE role = 'pending_approval';

-- +goose Down
DROP TABLE guild_memberships;
