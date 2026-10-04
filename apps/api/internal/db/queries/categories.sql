-- name: ListCategories :many
SELECT * FROM categories
WHERE (sqlc.narg('kind')::text IS NULL OR kind = sqlc.narg('kind'))
  AND (sqlc.arg('include_archived')::boolean OR archived_at IS NULL)
ORDER BY kind, archived_at IS NOT NULL, lower(name);

-- name: GetCategory :one
SELECT * FROM categories WHERE id = $1;

-- name: CreateCategory :one
INSERT INTO categories (name, kind, color, icon)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: UpdateCategory :one
UPDATE categories SET
    name = COALESCE(sqlc.narg('name'), name),
    color = CASE WHEN sqlc.arg('set_color')::boolean THEN sqlc.narg('color') ELSE color END,
    icon = CASE WHEN sqlc.arg('set_icon')::boolean THEN sqlc.narg('icon') ELSE icon END,
    archived_at = CASE
        WHEN sqlc.narg('archived')::boolean IS NULL THEN archived_at
        WHEN sqlc.narg('archived')::boolean THEN COALESCE(archived_at, now())
        ELSE NULL
    END
WHERE id = sqlc.arg('id')
RETURNING *;

-- name: DeleteCategory :execrows
DELETE FROM categories WHERE id = $1;
