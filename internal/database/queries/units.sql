-- name: GetKingdomUnits :many
SELECT * FROM kingdom_units
WHERE kingdom_id = $1;

-- name: GetAvailableKingdomUnits :many
-- Returns available unit counts for a kingdom via the kingdom_available_units view.
-- A unit is available if it is not committed to any campaign (any status).
SELECT unit_type, count FROM kingdom_available_units
WHERE kingdom_id = $1;

-- name: GetAvailableKingdomUnitsByIDs :many
-- Bulk version of GetAvailableKingdomUnits for the combat tick.
SELECT kingdom_id, unit_type, count FROM kingdom_available_units
WHERE kingdom_id = ANY(@ids::bigint[]);

-- name: GetAllKingdomUnits :many
SELECT * FROM kingdom_units
ORDER BY kingdom_id;

-- name: UpsertKingdomUnits :exec
INSERT INTO kingdom_units (kingdom_id, unit_type, count)
VALUES (@kingdom_id, @unit_type, @count)
ON CONFLICT (kingdom_id, unit_type)
DO UPDATE SET
    count      = kingdom_units.count + @count,
    updated_at = NOW();

-- name: DeductUnitCost :one
UPDATE kingdoms SET
    wood       = wood  - @wood_cost,
    stone      = stone - @stone_cost,
    mana       = mana  - @mana_cost,
    updated_at = NOW()
WHERE id = @kingdom_id
    AND wood  >= @wood_cost
    AND stone >= @stone_cost
    AND mana  >= @mana_cost
RETURNING id;

-- name: GetKingdomTraining :one
SELECT * FROM kingdom_training
WHERE kingdom_id = $1;

-- name: StartTraining :exec
INSERT INTO kingdom_training (kingdom_id, unit_type, count, ticks_remaining, ticks_total)
VALUES ($1, $2, $3, $4, $4);

-- name: DecrementAndListTrainingAtZero :many
WITH decremented AS (
    UPDATE kingdom_training
    SET ticks_remaining = ticks_remaining - 1
    WHERE ticks_remaining > 0
    RETURNING *
)
SELECT * FROM decremented WHERE ticks_remaining = 0;

-- name: DeleteTraining :exec
DELETE FROM kingdom_training
WHERE kingdom_id = $1;

-- name: CancelTrainingWithRefund :exec
-- Atomically deletes the training row and refunds the resource cost back to the
-- kingdom. The UPDATE runs only when a row was actually deleted (via the FROM
-- deleted join), so if the tick already completed training before this fires
-- the kingdom is not credited twice.
WITH deleted AS (
    DELETE FROM kingdom_training WHERE kingdom_id = @kingdom_id AND kingdom_training.id = @training_id RETURNING *
)
UPDATE kingdoms SET
    wood  = wood  + @wood_refund,
    stone = stone + @stone_refund,
    mana  = mana  + @mana_refund,
    updated_at = NOW()
FROM deleted
WHERE kingdoms.id = @kingdom_id;

-- name: DeductKingdomUnitsCasualties :exec
UPDATE kingdom_units
SET count = GREATEST(0, count - @casualties), updated_at = NOW()
WHERE kingdom_id = @kingdom_id
  AND unit_type = @unit_type;

-- name: BulkDeductKingdomUnitsCasualties :exec
UPDATE kingdom_units
SET count = GREATEST(0, count - data.casualties), updated_at = NOW()
FROM (
    SELECT
        unnest(@kingdom_ids::bigint[]) AS kingdom_id,
        unnest(@unit_types::text[])    AS unit_type,
        unnest(@casualties::int[])     AS casualties
) AS data
WHERE kingdom_units.kingdom_id = data.kingdom_id
  AND kingdom_units.unit_type  = data.unit_type;
