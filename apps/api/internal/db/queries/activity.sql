-- name: InsertActivity :exec
INSERT INTO activity_log (actor_id, channel, client_id, client_name, action, entity_type, entity_id, details)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8);

-- ListActivity returns the workspace's entries newest first. The cursor is
-- the (created_at, id) of the last entry of the previous page; callers ask
-- for one row more than the page size to know whether another page exists.
-- name: ListActivity :many
SELECT
    l.id, l.actor_id, l.channel, l.client_id, l.client_name, l.action,
    l.entity_type, l.entity_id, l.details, l.created_at,
    u.name AS actor_name,
    u.email AS actor_email
FROM activity_log l
LEFT JOIN users u ON u.id = l.actor_id
WHERE (sqlc.narg('actor_id')::uuid IS NULL OR l.actor_id = sqlc.narg('actor_id'))
  AND (sqlc.narg('channel')::text IS NULL OR l.channel = sqlc.narg('channel'))
  AND (sqlc.narg('entity_type')::text IS NULL OR l.entity_type = sqlc.narg('entity_type'))
  AND (sqlc.narg('entity_id')::uuid IS NULL OR l.entity_id = sqlc.narg('entity_id'))
  AND (sqlc.narg('before_created_at')::timestamptz IS NULL
       OR (l.created_at, l.id) < (sqlc.narg('before_created_at'), sqlc.narg('before_id')::uuid))
ORDER BY l.created_at DESC, l.id DESC
LIMIT @row_limit;
