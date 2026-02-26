-- name: CreateResource :one
INSERT INTO resources (wood, stone, food)
VALUES ($1, $2, $3)
RETURNING *;
-- name: ListResources :many
SELECT id,
  wood,
  stone,
  food
FROM resources
ORDER BY id ASC;
-- name: GetResource :one
SELECT *
FROM resources
WHERE id = $1;