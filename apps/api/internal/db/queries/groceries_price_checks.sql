-- Price checks: prices seen in a store without buying. Plain inserts and
-- deletes live in groceries_receipts.sql; these add the joined details.

-- name: GetPriceCheckDetail :one
SELECT c.id, c.store_id, s.name AS store_name, c.variant_id, v.product_id, p.name AS product_name,
       v.brand, v.pack_size, v.pack_label, p.compare_unit, p.display_multiplier,
       c.observed_on, c.price_minor, c.currency, c.note, c.created_at,
       (c.price_minor::numeric / coalesce(v.pack_size, 1))::float8 AS unit_price_minor
FROM price_checks c
JOIN grocery_stores s ON s.id = c.store_id
JOIN product_variants v ON v.id = c.variant_id
JOIN products p ON p.id = v.product_id
WHERE c.id = $1;

-- name: ListPriceCheckDetails :many
SELECT c.id, c.store_id, s.name AS store_name, c.variant_id, v.product_id, p.name AS product_name,
       v.brand, v.pack_size, v.pack_label, p.compare_unit, p.display_multiplier,
       c.observed_on, c.price_minor, c.currency, c.note, c.created_at,
       (c.price_minor::numeric / coalesce(v.pack_size, 1))::float8 AS unit_price_minor
FROM price_checks c
JOIN grocery_stores s ON s.id = c.store_id
JOIN product_variants v ON v.id = c.variant_id
JOIN products p ON p.id = v.product_id
WHERE (sqlc.narg('variant_id')::uuid IS NULL OR c.variant_id = sqlc.narg('variant_id'))
  AND (sqlc.narg('product_id')::uuid IS NULL OR v.product_id = sqlc.narg('product_id'))
  AND (sqlc.narg('store_id')::uuid IS NULL OR c.store_id = sqlc.narg('store_id'))
ORDER BY c.observed_on DESC, c.created_at DESC
LIMIT 200;
