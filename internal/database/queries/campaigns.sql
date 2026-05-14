-- name: GetCampaignsForKingdom :many
SELECT * FROM kingdom_campaigns
WHERE kingdom_id = $1
ORDER BY created_at ASC;

-- name: CreateCampaignIfAvailable :one
-- Atomically checks that enough units are available (via kingdom_available_units
-- view) and inserts the campaign in one statement. Returns no rows if the
-- available count is insufficient, which the caller maps to "not enough units".
WITH available AS (
    SELECT count FROM kingdom_available_units
    WHERE kingdom_id = @kingdom_id AND unit_type = @unit_type
)
INSERT INTO kingdom_campaigns (
    kingdom_id, target_kingdom_id, unit_type, count,
    action, ticks_remaining, action_ticks, travel_ticks
)
SELECT @kingdom_id, @target_kingdom_id, @unit_type, @send_count,
       @action, @ticks_remaining, @action_ticks, @travel_ticks
FROM available
WHERE available.count >= @send_count
RETURNING *;

-- name: AdvanceCampaignStatus :exec
UPDATE kingdom_campaigns
SET status = $2, ticks_remaining = $3
WHERE id = $1;

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

-- name: BulkUpdateCampaignCounts :exec
UPDATE kingdom_campaigns
SET count = data.count
FROM (
    SELECT unnest(@ids::bigint[]) AS id,
           unnest(@counts::int[]) AS count
) AS data
WHERE kingdom_campaigns.id = data.id;

-- name: DeleteCampaign :exec
DELETE FROM kingdom_campaigns
WHERE id = $1;

-- name: CancelCampaign :one
-- Atomically verifies ownership and non-returning status, then sets the campaign
-- to returning. Returns no rows if the campaign does not exist, belongs to a
-- different kingdom, or is already returning — all treated as a no-op by the caller.
UPDATE kingdom_campaigns
SET status = 'returning', ticks_remaining = travel_ticks
WHERE id = @id
  AND kingdom_id = @kingdom_id
  AND status != 'returning'
RETURNING id;
