-- name: GetKingdomByUserID :one
SELECT * FROM kingdoms
WHERE user_id = $1;

-- name: GetKingdomByID :one
SELECT * FROM kingdoms
WHERE id = $1;

-- name: GetKingdomByName :one
SELECT * FROM kingdoms
WHERE name = $1;

-- name: CreateKingdom :one
INSERT INTO kingdoms (user_id, name, x, y)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetKingdomsInViewport :many
SELECT id, name, x, y FROM kingdoms
WHERE x >= $1 AND x <= $2
  AND y >= $3 AND y <= $4;

-- name: UpdateKingdomAllocations :one
UPDATE kingdoms
SET
    wood_pct      = $2,
    stone_pct     = $3,
    food_pct      = $4,
    mana_pct      = $5,
    devotion_pct  = $6,
    knowledge_pct = $7,
    idle_pct      = $8,
    updated_at    = NOW()
WHERE user_id = $1
RETURNING *;

-- name: ListAllKingdoms :many
SELECT * FROM kingdoms;

-- name: GetKingdomsByIDs :many
SELECT * FROM kingdoms
WHERE id = ANY(@ids::bigint[]);

-- name: GetKingdomsByNames :many
SELECT * FROM kingdoms
WHERE name = ANY(@names::citext[]);

-- name: ListOtherKingdoms :many
SELECT id, name FROM kingdoms
WHERE id != $1
ORDER BY name;

-- name: BulkGainKingdomPopulation :exec
UPDATE kingdoms
SET population = population + data.gain
FROM (
    SELECT unnest(@ids::bigint[]) AS id,
           unnest(@gains::int[])  AS gain
) AS data
WHERE kingdoms.id = data.id;

-- name: StealKingdomPopulation :exec
UPDATE kingdoms
SET population = GREATEST(100, population - $2)
WHERE id = $1;

-- name: BulkTickKingdoms :exec
UPDATE kingdoms
SET
    wood       = new.wood,
    stone      = new.stone,
    food       = new.food,
    mana       = new.mana,
    devotion   = new.devotion,
    knowledge  = new.knowledge,
    population = new.population,
    updated_at = NOW()
FROM (
    SELECT
        unnest(@ids::bigint[])        AS id,
        unnest(@wood::bigint[])       AS wood,
        unnest(@stone::bigint[])      AS stone,
        unnest(@food::bigint[])       AS food,
        unnest(@mana::bigint[])       AS mana,
        unnest(@devotion::bigint[])   AS devotion,
        unnest(@knowledge::bigint[])  AS knowledge,
        unnest(@population::bigint[]) AS population
) AS new
WHERE kingdoms.id = new.id;
