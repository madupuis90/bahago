-- name: GetKingdomBuildings :many
SELECT * FROM kingdom_buildings
WHERE kingdom_id = $1;

-- name: GetAllKingdomBuildings :many
SELECT * FROM kingdom_buildings
ORDER BY kingdom_id;

-- name: GetKingdomConstruction :one
SELECT * FROM kingdom_constructions
WHERE kingdom_id = $1;

-- name: IncrementKingdomBuilding :exec
INSERT INTO kingdom_buildings (kingdom_id, building_type, count)
VALUES ($1, $2, 1)
ON CONFLICT (kingdom_id, building_type)
DO UPDATE SET
    count      = kingdom_buildings.count + 1,
    updated_at = NOW();

-- name: StartConstruction :exec
INSERT INTO kingdom_constructions (kingdom_id, building_type, ticks_remaining, ticks_total)
VALUES ($1, $2, $3, $3);

-- name: DecrementAndListCompleted :many
WITH decremented AS (
    UPDATE kingdom_constructions
    SET ticks_remaining = ticks_remaining - 1
    WHERE ticks_remaining > 0
    RETURNING *
)
SELECT * FROM decremented WHERE ticks_remaining = 0;

-- name: DeleteConstruction :exec
DELETE FROM kingdom_constructions
WHERE kingdom_id = $1;

-- name: DeductBuildingCost :one
UPDATE kingdoms SET
    wood      = wood      - @wood_cost,
    stone     = stone     - @stone_cost,
    food      = food      - @food_cost,
    mana      = mana      - @mana_cost,
    devotion  = devotion  - @devotion_cost,
    knowledge = knowledge - @knowledge_cost,
    updated_at = NOW()
WHERE id = @kingdom_id
    AND wood      >= @wood_cost
    AND stone     >= @stone_cost
    AND food      >= @food_cost
    AND mana      >= @mana_cost
    AND devotion  >= @devotion_cost
    AND knowledge >= @knowledge_cost
RETURNING id;
