-- name: InsertTick :one
INSERT INTO game_ticks (occurred_at) VALUES (NOW()) RETURNING id;

-- name: GetLatestTickID :one
SELECT id FROM game_ticks ORDER BY id DESC LIMIT 1;
