-- name: ListGroceryStores :many
SELECT * FROM grocery_stores
WHERE sqlc.arg('include_archived')::boolean OR archived_at IS NULL
ORDER BY archived_at IS NOT NULL, lower(name);

-- name: GetGroceryStore :one
SELECT * FROM grocery_stores WHERE id = $1;

-- name: CreateGroceryStore :one
INSERT INTO grocery_stores (name) VALUES ($1) RETURNING *;

-- name: UpdateGroceryStore :one
UPDATE grocery_stores SET
    name = COALESCE(sqlc.narg('name'), name),
    archived_at = CASE
        WHEN sqlc.narg('archived')::boolean IS NULL THEN archived_at
        WHEN sqlc.narg('archived')::boolean THEN COALESCE(archived_at, now())
        ELSE NULL
    END
WHERE id = sqlc.arg('id')
RETURNING *;

-- name: ListProductGroups :many
SELECT * FROM product_groups ORDER BY position, lower(name);

-- name: GetProductGroup :one
SELECT * FROM product_groups WHERE id = $1;

-- name: CountProductGroups :one
SELECT count(*) FROM product_groups;

-- name: SeedProductGroup :exec
INSERT INTO product_groups (name, is_food, color, position, default_key)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT DO NOTHING;

-- name: CreateProductGroup :one
INSERT INTO product_groups (name, is_food, color, position)
VALUES (
    sqlc.arg('name'), sqlc.arg('is_food'), sqlc.narg('color'),
    COALESCE(sqlc.narg('position')::int, (SELECT COALESCE(max(position) + 1, 0) FROM product_groups))
)
RETURNING *;

-- name: UpdateProductGroup :one
UPDATE product_groups SET
    name = COALESCE(sqlc.narg('name'), name),
    -- A different name makes the group custom: it loses its default key.
    default_key = CASE WHEN sqlc.narg('name')::text IS NOT NULL AND sqlc.narg('name')::text <> name THEN NULL ELSE default_key END,
    is_food = COALESCE(sqlc.narg('is_food'), is_food),
    color = CASE WHEN sqlc.arg('set_color')::boolean THEN sqlc.narg('color') ELSE color END,
    position = COALESCE(sqlc.narg('position')::int, position)
WHERE id = sqlc.arg('id')
RETURNING *;

-- name: ListGroceryProducts :many
SELECT p.*, g.name AS group_name, g.default_key AS group_default_key, g.color AS group_color,
       (SELECT count(*) FROM product_variants v WHERE v.product_id = p.id)::bigint AS variant_count
FROM products p
LEFT JOIN product_groups g ON g.id = p.group_id AND g.workspace_id = p.workspace_id
WHERE (sqlc.arg('include_archived')::boolean OR p.archived_at IS NULL)
  AND (sqlc.narg('group_id')::uuid IS NULL OR p.group_id = sqlc.narg('group_id'))
ORDER BY p.archived_at IS NOT NULL, lower(p.name);

-- name: GetGroceryProduct :one
SELECT p.*, g.name AS group_name, g.default_key AS group_default_key, g.color AS group_color,
       (SELECT count(*) FROM product_variants v WHERE v.product_id = p.id)::bigint AS variant_count
FROM products p
LEFT JOIN product_groups g ON g.id = p.group_id AND g.workspace_id = p.workspace_id
WHERE p.id = $1;

-- name: CreateGroceryProduct :one
INSERT INTO products (name, group_id, compare_unit, display_multiplier)
VALUES ($1, $2, $3, $4)
RETURNING id;

-- name: UpdateGroceryProduct :one
UPDATE products SET
    name = COALESCE(sqlc.narg('name'), name),
    group_id = CASE WHEN sqlc.arg('set_group')::boolean THEN sqlc.narg('group_id') ELSE group_id END,
    compare_unit = COALESCE(sqlc.narg('compare_unit'), compare_unit),
    display_multiplier = COALESCE(sqlc.narg('display_multiplier')::int, display_multiplier),
    archived_at = CASE
        WHEN sqlc.narg('archived')::boolean IS NULL THEN archived_at
        WHEN sqlc.narg('archived')::boolean THEN COALESCE(archived_at, now())
        ELSE NULL
    END
WHERE id = sqlc.arg('id')
RETURNING id;

-- name: ListProductVariants :many
SELECT * FROM product_variants WHERE product_id = $1
ORDER BY lower(brand), pack_size NULLS FIRST;

-- name: GetProductVariant :one
SELECT * FROM product_variants WHERE id = $1;

-- name: CreateProductVariant :one
INSERT INTO product_variants (product_id, brand, pack_size, pack_label, store_id)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: UpdateProductVariant :one
UPDATE product_variants SET
    brand = COALESCE(sqlc.narg('brand'), brand),
    pack_size = CASE WHEN sqlc.arg('set_pack_size')::boolean THEN sqlc.narg('pack_size') ELSE pack_size END,
    pack_label = CASE WHEN sqlc.arg('set_pack_label')::boolean THEN sqlc.narg('pack_label') ELSE pack_label END,
    store_id = CASE WHEN sqlc.arg('set_store')::boolean THEN sqlc.narg('store_id') ELSE store_id END
WHERE id = sqlc.arg('id')
RETURNING *;

-- name: DeleteProductVariant :execrows
DELETE FROM product_variants WHERE id = $1;
