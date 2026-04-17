-- name: GetKingdomUnits :many
SELECT * FROM kingdom_units
WHERE kingdom_id = $1;

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

-- name: DecrementAndListCompletedTraining :many
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
