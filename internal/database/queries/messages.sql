-- name: ListInboxMessages :many
SELECT
    m.id,
    m.from_kingdom_id,
    m.to_kingdom_id,
    m.subject,
    m.body,
    m.read_at,
    m.created_at,
    fk.name AS from_kingdom_name
FROM kingdom_messages m
JOIN kingdoms fk ON fk.id = m.from_kingdom_id
WHERE m.to_kingdom_id = $1
  AND m.deleted_at IS NULL
ORDER BY m.created_at DESC
LIMIT 50;

-- name: GetInboxMessageByID :one
SELECT
    m.id,
    m.from_kingdom_id,
    m.to_kingdom_id,
    m.subject,
    m.body,
    m.action_url,
    m.action_text,
    m.read_at,
    m.created_at,
    fk.name AS from_kingdom_name,
    tk.name AS to_kingdom_name
FROM kingdom_messages m
JOIN kingdoms fk ON fk.id = m.from_kingdom_id
JOIN kingdoms tk ON tk.id = m.to_kingdom_id
WHERE m.id = $1
  AND m.to_kingdom_id = $2
  AND m.deleted_at IS NULL;

-- name: BulkCreateMessages :exec
INSERT INTO kingdom_messages (from_kingdom_id, to_kingdom_id, subject, body, action_url, action_text)
SELECT @from_kingdom_id::bigint, unnest(@to_kingdom_ids::bigint[]), @subject::text, @body::text, @action_url::text, @action_text::text;

-- name: MarkMessageRead :exec
UPDATE kingdom_messages
SET read_at = NOW()
WHERE id = $1
  AND to_kingdom_id = $2
  AND read_at IS NULL;

-- name: DeleteMessage :exec
UPDATE kingdom_messages
SET deleted_at = NOW()
WHERE id = $1
  AND to_kingdom_id = $2;

-- name: CountUnreadMessages :one
SELECT COUNT(*)
FROM kingdom_messages
WHERE to_kingdom_id = $1
  AND read_at IS NULL
  AND deleted_at IS NULL;
