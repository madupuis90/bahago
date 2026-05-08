-- name: ListKingdomPrayers :many
SELECT * FROM kingdom_prayers
WHERE kingdom_id = $1
ORDER BY started_at ASC;

-- name: ListPrayersTargetingKingdom :many
SELECT * FROM kingdom_prayers
WHERE target_kingdom_id = $1
ORDER BY started_at ASC;

-- name: CreatePrayer :one
INSERT INTO kingdom_prayers (kingdom_id, prayer_type, target_kingdom_id, ticks_remaining, ticks_total)
VALUES (@kingdom_id, @prayer_type, @target_kingdom_id, @ticks_total, @ticks_total)
RETURNING *;

-- name: DeletePrayer :exec
DELETE FROM kingdom_prayers
WHERE id = @id AND kingdom_id = @kingdom_id;

-- name: GetAllKingdomPrayers :many
SELECT * FROM kingdom_prayers;

-- name: DecrementAndListPrayersAtZero :many
WITH expired AS (
    DELETE FROM kingdom_prayers
    WHERE ticks_remaining = 1
    RETURNING *
),
decremented AS (
    UPDATE kingdom_prayers
    SET ticks_remaining = ticks_remaining - 1
    WHERE ticks_remaining > 1
)
SELECT * FROM expired;

-- name: DeleteKingdomPrayers :exec
DELETE FROM kingdom_prayers
WHERE kingdom_id = ANY(@ids::bigint[]);
