-- name: CreateGuild :one
WITH new_guild AS (
    INSERT INTO guilds (name, slug, description)
    VALUES (@name, @slug, @description)
    RETURNING *
), insert_membership AS (
    INSERT INTO guild_memberships (guild_id, kingdom_id, role)
    SELECT id, @founder_kingdom_id::bigint, 'applicant'
    FROM new_guild
)
SELECT * FROM new_guild;

-- name: GetGuildBySlug :one
SELECT * FROM guilds WHERE slug = $1 LIMIT 1;

-- name: GetGuildByID :one
SELECT * FROM guilds WHERE id = $1 LIMIT 1;

-- name: UpdateGuildDescription :exec
UPDATE guilds
SET description = @description, updated_at = NOW()
WHERE id = @id;

-- name: CountGuildSupporters :one
SELECT COUNT(*) FROM guild_memberships
WHERE guild_id = $1 AND role IN ('applicant', 'supporter');

-- name: ActivateGuild :exec
WITH activate AS (
    UPDATE guilds
    SET status = 'active',
        founding_kingdom_ids = (
            SELECT array_agg(gm2.kingdom_id)
            FROM guild_memberships gm2
            WHERE gm2.guild_id = $1
              AND gm2.role IN ('applicant', 'supporter')
        ),
        updated_at = NOW()
    WHERE guilds.id = $1
)
UPDATE guild_memberships
SET role = CASE WHEN guild_memberships.role = 'applicant' THEN 'leader' ELSE 'member' END,
    joined_at = NOW()
WHERE guild_memberships.guild_id = $1
  AND guild_memberships.role IN ('applicant', 'supporter');

-- name: DisbandGuild :exec
DELETE FROM guilds WHERE id = $1;

-- name: CancelProposal :exec
DELETE FROM guilds WHERE id = $1 AND status = 'pending';

-- name: ExpirePendingGuilds :exec
DELETE FROM guilds
WHERE status = 'pending' AND created_at < NOW() - INTERVAL '7 days';

-- name: GetKingdomGuildMembership :one
SELECT gm.*, g.slug AS guild_slug, g.name AS guild_name
FROM guild_memberships gm
JOIN guilds g ON g.id = gm.guild_id
WHERE gm.kingdom_id = $1
  AND gm.role IN ('applicant', 'supporter', 'member', 'officer', 'leader')
LIMIT 1;

-- name: GetMembershipByID :one
SELECT * FROM guild_memberships WHERE id = $1 AND guild_id = $2 LIMIT 1;

-- name: GetMembershipByKingdomAndGuild :one
SELECT * FROM guild_memberships
WHERE kingdom_id = $1 AND guild_id = $2
LIMIT 1;

-- name: CreateGuildMembership :exec
INSERT INTO guild_memberships (guild_id, kingdom_id, role)
VALUES (@guild_id, @kingdom_id, @role);



-- name: ListGuildMembersWithNames :many
SELECT gm.id, gm.guild_id, gm.kingdom_id, gm.role, gm.joined_at, gm.created_at,
       k.name AS kingdom_name
FROM guild_memberships gm
JOIN kingdoms k ON k.id = gm.kingdom_id
WHERE gm.guild_id = $1 AND gm.role IN ('applicant', 'supporter', 'member', 'officer', 'leader')
ORDER BY
    CASE gm.role
        WHEN 'leader'    THEN 1
        WHEN 'applicant' THEN 1
        WHEN 'officer'   THEN 2
        WHEN 'member'    THEN 3
        WHEN 'supporter' THEN 4
    END,
    gm.joined_at ASC NULLS LAST,
    gm.created_at ASC;

-- name: ListPendingRequests :many
SELECT gm.id, gm.guild_id, gm.kingdom_id, gm.role, gm.joined_at, gm.created_at,
       k.name AS kingdom_name
FROM guild_memberships gm
JOIN kingdoms k ON k.id = gm.kingdom_id
WHERE gm.guild_id = $1 AND gm.role = 'pending_approval'
ORDER BY gm.created_at ASC;

-- name: ApproveMembershipIfNotFull :one
-- Atomically checks the active member count and approves the pending request
-- in one statement. Returns no rows if the guild is full (>= 20 active members)
-- or the request no longer exists, which the caller maps to the appropriate error.
WITH member_count AS (
    SELECT COUNT(*) AS cnt
    FROM guild_memberships
    WHERE guild_memberships.guild_id = @guild_id AND guild_memberships.role IN ('member', 'officer', 'leader')
),
approved AS (
    UPDATE guild_memberships
    SET role = 'member', joined_at = NOW()
    WHERE guild_memberships.id = @membership_id
      AND guild_memberships.guild_id = @guild_id
      AND guild_memberships.role = 'pending_approval'
      AND (SELECT cnt FROM member_count) < 20
    RETURNING guild_memberships.kingdom_id
),
cleanup AS (
    DELETE FROM guild_memberships gm2
    WHERE gm2.kingdom_id = (SELECT kingdom_id FROM approved)
      AND gm2.role = 'pending_approval'
      AND gm2.guild_id != @guild_id
),
cancel_invitations AS (
    DELETE FROM guild_memberships gm3
    WHERE gm3.kingdom_id = (SELECT kingdom_id FROM approved)
      AND gm3.role = 'invited'
)
SELECT kingdom_id FROM approved;

-- name: RejectMembership :exec
DELETE FROM guild_memberships
WHERE id = @membership_id AND guild_id = @guild_id AND role = 'pending_approval';

-- name: RemoveMembership :exec
DELETE FROM guild_memberships
WHERE kingdom_id = @kingdom_id AND guild_id = @guild_id
  AND role IN ('member', 'officer', 'supporter');

-- name: SetMembershipRole :exec
UPDATE guild_memberships
SET role = @role
WHERE kingdom_id = @kingdom_id AND guild_id = @guild_id
  AND role IN ('member', 'officer');

-- name: TransferLeadership :exec
UPDATE guild_memberships
SET role = CASE
    WHEN kingdom_id = @new_leader_kingdom_id::bigint THEN 'leader'
    ELSE 'member'
END
WHERE guild_id = @guild_id
  AND (
    (kingdom_id = @new_leader_kingdom_id::bigint AND role IN ('member', 'officer'))
    OR role = 'leader'
  );

-- name: WithdrawSupport :exec
DELETE FROM guild_memberships
WHERE kingdom_id = $1 AND guild_id = $2 AND role = 'supporter';

-- name: CancelJoinRequest :exec
DELETE FROM guild_memberships
WHERE kingdom_id = $1 AND guild_id = $2 AND role = 'pending_approval';

-- name: CancelOtherPendingRequests :exec
DELETE FROM guild_memberships
WHERE kingdom_id = @kingdom_id AND guild_id != @guild_id AND role = 'pending_approval';

-- name: RequestJoinIfNotFull :one
-- Atomically checks the active member count and inserts a pending_approval row
-- in one statement. Returns no rows if the guild is full (>= 20 active members),
-- which the caller maps to "this guild is full".
WITH member_count AS (
    SELECT COUNT(*) AS cnt
    FROM guild_memberships
    WHERE guild_id = @guild_id AND role IN ('member', 'officer', 'leader')
)
INSERT INTO guild_memberships (guild_id, kingdom_id, role)
SELECT @guild_id, @kingdom_id, 'pending_approval'
FROM member_count
WHERE cnt < 20
RETURNING id, guild_id, kingdom_id, role, joined_at, created_at;

-- name: CreateGuildInvitation :exec
INSERT INTO guild_memberships (guild_id, kingdom_id, role)
VALUES (@guild_id, @kingdom_id, 'invited');

-- name: ListGuildInvitations :many
SELECT gm.id, gm.guild_id, gm.kingdom_id, gm.created_at,
       k.name AS kingdom_name
FROM guild_memberships gm
JOIN kingdoms k ON k.id = gm.kingdom_id
WHERE gm.guild_id = $1 AND gm.role = 'invited'
ORDER BY gm.created_at DESC;

-- name: ListKingdomInvitations :many
SELECT gm.id, gm.guild_id, gm.kingdom_id, gm.created_at,
       g.name AS guild_name,
       g.slug AS guild_slug
FROM guild_memberships gm
JOIN guilds g ON g.id = gm.guild_id
WHERE gm.kingdom_id = $1 AND gm.role = 'invited'
ORDER BY gm.created_at DESC;

-- name: GetKingdomGuildInvitation :one
SELECT id FROM guild_memberships WHERE kingdom_id = $1 AND guild_id = $2 AND role = 'invited' LIMIT 1;

-- name: RevokeGuildInvitation :one
DELETE FROM guild_memberships WHERE id = $1 AND guild_id = $2 AND role = 'invited' RETURNING kingdom_id;

-- name: DeclineGuildInvitation :one
DELETE FROM guild_memberships WHERE id = $1 AND kingdom_id = $2 AND role = 'invited' RETURNING guild_id;

-- name: AcceptGuildInvitation :one
-- Atomically promotes the invited row to member if the guild is not full (< 20),
-- cancels any pending join requests for the kingdom in other guilds, and
-- cancels any other outstanding invitations. Returns no rows if the invitation
-- is not found or the guild is at capacity.
WITH member_count AS (
    SELECT COUNT(*) AS cnt
    FROM guild_memberships gm
    WHERE gm.guild_id = @guild_id AND gm.role IN ('member', 'officer', 'leader')
),
new_member AS (
    UPDATE guild_memberships AS gm_inv
    SET role = 'member', joined_at = NOW()
    WHERE gm_inv.id = @invitation_id
      AND gm_inv.kingdom_id = @kingdom_id
      AND gm_inv.guild_id = @guild_id
      AND gm_inv.role = 'invited'
      AND (SELECT cnt FROM member_count) < 20
    RETURNING gm_inv.kingdom_id
),
cancel_requests AS (
    DELETE FROM guild_memberships gm2
    WHERE gm2.kingdom_id = (SELECT kingdom_id FROM new_member)
      AND gm2.role = 'pending_approval'
),
cancel_invitations AS (
    DELETE FROM guild_memberships gm3
    WHERE gm3.kingdom_id = (SELECT kingdom_id FROM new_member)
      AND gm3.role = 'invited'
      AND gm3.guild_id != @guild_id
)
SELECT kingdom_id FROM new_member;

-- name: ListGuildMembersExcludingSelf :many
SELECT kingdom_id
FROM guild_memberships
WHERE guild_id = @guild_id
  AND kingdom_id != @exclude_kingdom_id
  AND role IN ('member', 'officer', 'leader');

-- name: ListGuildOfficersExcludingSelf :many
SELECT kingdom_id
FROM guild_memberships
WHERE guild_id = @guild_id
  AND kingdom_id != @exclude_kingdom_id
  AND role IN ('officer', 'leader');

-- name: UpdateGuildSettings :exec
UPDATE guilds
SET settings = @settings, updated_at = NOW()
WHERE id = @id;

-- name: GetGuildsForKingdoms :many
-- The active guild commitment for each listed kingdom (one per kingdom via
-- guild_memberships_one_active_per_kingdom). applicant/supporter are included
-- to match GetKingdomGuildMembership's notion of a kingdom's guild; applicant
-- rows have not yet joined but signal the intended affiliation.
SELECT
    gm.kingdom_id,
    g.id   AS guild_id,
    g.slug AS guild_slug,
    g.name AS guild_name
FROM guild_memberships gm
JOIN guilds g ON g.id = gm.guild_id
WHERE gm.kingdom_id = ANY(@kingdom_ids::bigint[])
  AND gm.role IN ('applicant', 'supporter', 'member', 'officer', 'leader');

-- name: ListActiveGuilds :many
SELECT
    g.name,
    g.slug,
    k.name AS leader_name,
    COUNT(m.id)::int AS member_count
FROM guilds g
LEFT JOIN guild_memberships l ON l.guild_id = g.id AND l.role = 'leader'
LEFT JOIN kingdoms k ON k.id = l.kingdom_id
LEFT JOIN guild_memberships m ON m.guild_id = g.id AND m.role IN ('member', 'officer', 'leader')
WHERE g.status = 'active'
GROUP BY g.id, g.name, g.slug, k.name
ORDER BY g.name ASC;

-- name: ListPendingGuilds :many
SELECT
    g.name,
    g.slug,
    k.name AS founder_name,
    COUNT(p.id)::int AS supporter_count,
    (g.created_at + INTERVAL '7 days')::timestamptz AS expires_at
FROM guilds g
LEFT JOIN guild_memberships a ON a.guild_id = g.id AND a.role = 'applicant'
LEFT JOIN kingdoms k ON k.id = a.kingdom_id
LEFT JOIN guild_memberships p ON p.guild_id = g.id AND p.role IN ('applicant', 'supporter')
WHERE g.status = 'pending'
GROUP BY g.id, g.name, g.slug, k.name, g.created_at
ORDER BY g.name ASC;
