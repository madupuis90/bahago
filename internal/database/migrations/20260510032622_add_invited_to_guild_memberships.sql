-- +goose Up
ALTER TABLE guild_memberships
    DROP CONSTRAINT guild_memberships_role_valid,
    ADD CONSTRAINT guild_memberships_role_valid
        CHECK (role IN ('applicant', 'supporter', 'pending_approval', 'invited', 'member', 'officer', 'leader'));

DROP INDEX guild_memberships_one_pending_per_kingdom_per_guild;

-- Prevents a kingdom from having both a join request and an invitation for the same guild.
CREATE UNIQUE INDEX guild_memberships_one_pending_or_invited_per_kingdom_per_guild
    ON guild_memberships (kingdom_id, guild_id)
    WHERE role IN ('pending_approval', 'invited');

-- +goose Down
ALTER TABLE guild_memberships
    DROP CONSTRAINT guild_memberships_role_valid,
    ADD CONSTRAINT guild_memberships_role_valid
        CHECK (role IN ('applicant', 'supporter', 'pending_approval', 'member', 'officer', 'leader'));

DROP INDEX guild_memberships_one_pending_or_invited_per_kingdom_per_guild;

CREATE UNIQUE INDEX guild_memberships_one_pending_per_kingdom_per_guild
    ON guild_memberships (kingdom_id, guild_id)
    WHERE role = 'pending_approval';
