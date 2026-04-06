-- name: GetKingdomByUserID :one
SELECT * FROM kingdoms
WHERE user_id = $1;

-- name: CreateKingdom :one
INSERT INTO kingdoms (user_id, name)
VALUES ($1, $2)
RETURNING *;

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
