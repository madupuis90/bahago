-- name: GetCampaignsForKingdom :many
SELECT
    kc.id,
    kc.kingdom_id,
    kc.target_kingdom_id,
    kc.legion_id,
    kl.name    AS legion_name,
    kl.number  AS legion_number,
    kc.action,
    kc.status,
    kc.ticks_remaining,
    kc.action_ticks,
    kc.travel_ticks,
    kc.created_at
FROM kingdom_campaigns kc
JOIN kingdom_legions kl ON kl.id = kc.legion_id
WHERE kc.kingdom_id = $1
ORDER BY kc.created_at ASC;

-- name: ListCampaignUnitsForKingdom :many
SELECT kcu.campaign_id, kcu.unit_type, kcu.count
FROM kingdom_campaign_units kcu
JOIN kingdom_campaigns kc ON kc.id = kcu.campaign_id
WHERE kc.kingdom_id = $1;

-- name: GetCampaignUnitsByCampaignIDs :many
SELECT * FROM kingdom_campaign_units
WHERE campaign_id = ANY(@ids::bigint[]);

-- name: IsLegionDeployed :one
SELECT EXISTS(
    SELECT 1 FROM kingdom_campaigns WHERE legion_id = $1
) AS deployed;

-- name: CreateCampaign :one
INSERT INTO kingdom_campaigns (
    kingdom_id, target_kingdom_id, legion_id,
    action, ticks_remaining, action_ticks, travel_ticks
)
VALUES (
    @kingdom_id, @target_kingdom_id, @legion_id,
    @action, @ticks_remaining, @action_ticks, @travel_ticks
)
RETURNING *;

-- name: SnapshotLegionUnitsIntoCampaign :exec
INSERT INTO kingdom_campaign_units (campaign_id, unit_type, count)
SELECT @campaign_id, unit_type, count
FROM kingdom_legion_units
WHERE legion_id = @legion_id;

-- name: DecrementAndListCampaignsAtZero :many
WITH decremented AS (
    UPDATE kingdom_campaigns
    SET ticks_remaining = ticks_remaining - 1
    WHERE ticks_remaining > 0
    RETURNING *
)
SELECT * FROM decremented WHERE ticks_remaining = 0;

-- name: GetActiveCampaignsReadyForCombat :many
SELECT * FROM kingdom_campaigns
WHERE status = 'active'
  AND ticks_remaining > 0;

-- name: BulkActivateCampaigns :exec
UPDATE kingdom_campaigns
SET status = 'active', ticks_remaining = action_ticks
WHERE id = ANY(@ids::bigint[]);

-- name: BulkReturnCampaigns :exec
UPDATE kingdom_campaigns
SET status = 'returning', ticks_remaining = travel_ticks
WHERE id = ANY(@ids::bigint[]);

-- name: BulkDeleteCampaigns :exec
DELETE FROM kingdom_campaigns
WHERE id = ANY(@ids::bigint[]);

-- name: BulkUpdateCampaignUnitCounts :exec
UPDATE kingdom_campaign_units
SET count = data.count
FROM (
    SELECT
        unnest(@campaign_ids::bigint[]) AS campaign_id,
        unnest(@unit_types::text[])     AS unit_type,
        unnest(@counts::int[])          AS count
) AS data
WHERE kingdom_campaign_units.campaign_id = data.campaign_id
  AND kingdom_campaign_units.unit_type   = data.unit_type;

-- name: BulkDeleteCampaignUnitsZero :exec
DELETE FROM kingdom_campaign_units
WHERE (campaign_id, unit_type) IN (
    SELECT unnest(@campaign_ids::bigint[]), unnest(@unit_types::text[])
);

-- name: BulkRestoreLegionUnits :exec
INSERT INTO kingdom_legion_units (legion_id, unit_type, count)
SELECT kc.legion_id, kcu.unit_type, kcu.count
FROM kingdom_campaign_units kcu
JOIN kingdom_campaigns kc ON kc.id = kcu.campaign_id
WHERE kc.id = ANY(@ids::bigint[])
ON CONFLICT (legion_id, unit_type)
DO UPDATE SET count = kingdom_legion_units.count + EXCLUDED.count;

-- name: CancelCampaign :one
-- Atomically verifies ownership and non-returning status, then sets the campaign
-- to returning. Returns no rows if the campaign does not exist, belongs to a
-- different kingdom, or is already returning — all treated as a no-op by the caller.
-- When en_route, the return trip is proportional to distance already traveled;
-- when active (at target), the full travel_ticks applies.
UPDATE kingdom_campaigns
SET status = 'returning',
    ticks_remaining = CASE
        WHEN status = 'en_route' THEN GREATEST(travel_ticks - ticks_remaining, 1)
        ELSE travel_ticks
    END
WHERE id = @id
  AND kingdom_id = @kingdom_id
  AND status != 'returning'
RETURNING id;
