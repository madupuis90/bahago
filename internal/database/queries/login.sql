-- name: CreateUser :one
INSERT INTO users (email, pw_hash)
VALUES ($1, $2)
RETURNING id;

-- name: GetUserByEmail :one
SELECT * FROM users
WHERE email = $1
LIMIT 1;

-- name: UpdateLastLogin :exec
UPDATE users
SET last_login_at = NOW()
WHERE id = $1;

-- name: CreateSession :one
INSERT INTO sessions (
    id, user_id, ip_address, user_agent, expires_at
) VALUES (
    $1, $2, $3, $4, $5
)
RETURNING *;

-- name: GetUserBySessionID :one
SELECT 
    u.id, 
    u.email, 
    u.is_active, 
    u.is_verified,
    s.id AS session_id,
    s.expires_at
FROM sessions s
JOIN users u ON s.user_id = u.id
WHERE s.id = $1 
  AND s.expires_at > CURRENT_TIMESTAMP;