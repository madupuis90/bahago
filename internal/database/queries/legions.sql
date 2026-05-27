-- name: ListLegionsForKingdom :many
SELECT
    kl.id,
    kl.kingdom_id,
    kl.number,
    kl.name,
    kl.created_at,
    kc.status AS campaign_status
FROM kingdom_legions kl
LEFT JOIN kingdom_campaigns kc ON kc.legion_id = kl.id
WHERE kl.kingdom_id = $1
ORDER BY kl.number;

-- name: ListAllLegionUnitsForKingdom :many
SELECT klu.legion_id, klu.unit_type, klu.count
FROM kingdom_legion_units klu
JOIN kingdom_legions kl ON kl.id = klu.legion_id
WHERE kl.kingdom_id = $1
ORDER BY klu.legion_id, klu.unit_type;

-- name: GetLegion :one
SELECT * FROM kingdom_legions
WHERE id = $1 AND kingdom_id = $2;

-- name: GetLegionForUpdate :one
SELECT * FROM kingdom_legions
WHERE id = $1 AND kingdom_id = $2
FOR UPDATE;

-- name: CountLegionsForKingdom :one
SELECT COUNT(*) FROM kingdom_legions WHERE kingdom_id = $1;

-- name: CreateLegion :one
INSERT INTO kingdom_legions (kingdom_id, number, name)
SELECT @kingdom_id, s, 'Legion ' || s
FROM generate_series(1, @cap::int) s
WHERE s NOT IN (
    SELECT number FROM kingdom_legions WHERE kingdom_id = @kingdom_id
)
ORDER BY s
LIMIT 1
RETURNING *;

-- name: DeleteLegion :exec
DELETE FROM kingdom_legions WHERE id = $1 AND kingdom_id = $2;

-- name: UpsertLegionUnit :exec
INSERT INTO kingdom_legion_units (legion_id, unit_type, count)
VALUES (@legion_id, @unit_type, @count)
ON CONFLICT (legion_id, unit_type)
DO UPDATE SET count = kingdom_legion_units.count + EXCLUDED.count;

-- name: DecrementLegionUnit :exec
WITH updated AS (
    UPDATE kingdom_legion_units
    SET count = count - @amount::int
    WHERE legion_id = @legion_id AND unit_type = @unit_type
      AND count - @amount > 0
    RETURNING legion_id, unit_type
)
DELETE FROM kingdom_legion_units AS klu
WHERE klu.legion_id = @legion_id
  AND klu.unit_type = @unit_type
  AND NOT EXISTS (SELECT 1 FROM updated);

-- name: ListLegionUnits :many
SELECT * FROM kingdom_legion_units WHERE legion_id = $1;

-- name: ClearLegionUnits :exec
DELETE FROM kingdom_legion_units WHERE legion_id = $1;

-- name: BulkUpdateLegionUnitCounts :exec
UPDATE kingdom_legion_units
SET count = data.count
FROM (
    SELECT
        unnest(@legion_ids::bigint[]) AS legion_id,
        unnest(@unit_types::text[])   AS unit_type,
        unnest(@counts::int[])        AS count
) AS data
WHERE kingdom_legion_units.legion_id = data.legion_id
  AND kingdom_legion_units.unit_type = data.unit_type;

-- name: BulkDeleteLegionUnitsZero :exec
DELETE FROM kingdom_legion_units
WHERE (legion_id, unit_type) IN (
    SELECT unnest(@legion_ids::bigint[]), unnest(@unit_types::text[])
);

-- name: GetAtHomeLegionUnitsByKingdomIDs :many
SELECT klu.legion_id, kl.kingdom_id, klu.unit_type, klu.count
FROM kingdom_legion_units klu
JOIN kingdom_legions kl ON kl.id = klu.legion_id
WHERE kl.kingdom_id = ANY(@ids::bigint[])
  AND NOT EXISTS (
      SELECT 1 FROM kingdom_campaigns kc WHERE kc.legion_id = klu.legion_id
  );
