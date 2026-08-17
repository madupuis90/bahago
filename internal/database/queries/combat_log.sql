-- name: InsertCombatLog :one
INSERT INTO kingdom_combat_log (
    tick_id,
    target_kingdom_id,
    attacker_units, defender_units,
    attacker_power, defender_power,
    winner,
    attacker_casualties, defender_casualties,
    population_stolen
) VALUES (
    @tick_id, @target_kingdom_id,
    @attacker_units, @defender_units,
    @attacker_power, @defender_power,
    @winner,
    @attacker_casualties, @defender_casualties,
    @population_stolen
)
RETURNING id;

-- name: BulkInsertCombatLogParticipants :exec
INSERT INTO kingdom_combat_log_participants (combat_log_id, kingdom_id, role, population_gained)
SELECT @combat_log_id, data.kingdom_id, data.role, data.population_gained
FROM (
    SELECT
        unnest(@kingdom_ids::bigint[]) AS kingdom_id,
        unnest(@roles::text[])         AS role,
        unnest(@population_gained::int[]) AS population_gained
) AS data;

-- name: GetRecentCombatLogs :many
WITH my_log_ids AS (
    SELECT l.id, l.occurred_at
    FROM kingdom_combat_log l
    JOIN kingdom_combat_log_participants p ON l.id = p.combat_log_id
    WHERE p.kingdom_id = $1
    ORDER BY l.occurred_at DESC
    LIMIT 5
)
SELECT l.id, l.tick_id, l.target_kingdom_id, l.attacker_units, l.defender_units,
       l.attacker_power, l.defender_power, l.winner,
       l.attacker_casualties, l.defender_casualties, l.population_stolen, l.occurred_at,
       p.kingdom_id AS participant_kingdom_id,
       k.name       AS participant_name,
       p.role       AS participant_role,
       p.population_gained
FROM kingdom_combat_log l
JOIN my_log_ids ml ON l.id = ml.id
JOIN kingdom_combat_log_participants p ON l.id = p.combat_log_id
JOIN kingdoms k ON p.kingdom_id = k.id
ORDER BY l.occurred_at DESC, l.id, p.role, k.name;
