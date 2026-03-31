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

-- name: CreateEmailVerification :exec
INSERT INTO email_verification_tokens (token, user_id, expires_at)
VALUES ($1, $2, $3);

-- name: ConsumeEmailVerification :one
DELETE FROM email_verification_tokens
WHERE token = $1
  AND expires_at > NOW()
RETURNING user_id;

-- name: VerifyUser :exec
UPDATE users
SET is_verified = true
WHERE id = $1;

-- name: DeleteSession :exec
DELETE FROM sessions WHERE id = $1;

-- name: DeleteSessionsByUserID :exec
DELETE FROM sessions WHERE user_id = $1;

-- name: DeletePasswordResetTokensByUserID :exec
DELETE FROM password_reset_tokens WHERE user_id = $1;

-- name: CreatePasswordResetToken :exec
INSERT INTO password_reset_tokens (token, user_id, expires_at)
VALUES ($1, $2, $3);

-- name: ConsumePasswordResetToken :one
DELETE FROM password_reset_tokens
WHERE token = $1
  AND expires_at > NOW()
RETURNING user_id;

-- name: UpdatePassword :exec
UPDATE users
SET pw_hash = $2, updated_at = NOW()
WHERE id = $1;
