-- Plain statements for receipts, lines, aliases and price checks. Business
-- rules live in the service.

-- name: InsertGroceryReceipt :one
INSERT INTO grocery_receipts (store_id, transaction_id, purchased_on, currency, total_minor, source, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetGroceryReceipt :one
SELECT * FROM grocery_receipts WHERE id = $1;

-- name: ListGroceryReceipts :many
SELECT * FROM grocery_receipts
WHERE (sqlc.narg('store_id')::uuid IS NULL OR store_id = sqlc.narg('store_id'))
  AND (sqlc.narg('from_date')::date IS NULL OR purchased_on >= sqlc.narg('from_date'))
  AND (sqlc.narg('to_date')::date IS NULL OR purchased_on <= sqlc.narg('to_date'))
ORDER BY purchased_on DESC, created_at DESC;

-- name: DeleteGroceryReceipt :execrows
DELETE FROM grocery_receipts WHERE id = $1;

-- name: InsertGroceryReceiptLine :one
INSERT INTO grocery_receipt_lines (receipt_id, position, raw_text, quantity, amount_minor, discount_minor, variant_id)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: ListGroceryReceiptLines :many
SELECT * FROM grocery_receipt_lines WHERE receipt_id = $1 ORDER BY position;

-- name: SetGroceryReceiptLineVariant :one
UPDATE grocery_receipt_lines SET variant_id = sqlc.narg('variant_id')
WHERE id = sqlc.arg('id')
RETURNING *;

-- name: UpsertStoreItemAlias :one
INSERT INTO store_item_aliases (store_id, raw_text_key, variant_id)
VALUES ($1, $2, $3)
ON CONFLICT (workspace_id, store_id, raw_text_key) DO UPDATE SET variant_id = EXCLUDED.variant_id
RETURNING *;

-- name: GetStoreItemAlias :one
SELECT * FROM store_item_aliases WHERE store_id = $1 AND raw_text_key = $2;

-- name: ListStoreItemAliases :many
SELECT * FROM store_item_aliases WHERE store_id = $1 ORDER BY raw_text_key;

-- name: DeleteStoreItemAlias :execrows
DELETE FROM store_item_aliases WHERE id = $1;

-- name: InsertPriceCheck :one
INSERT INTO price_checks (store_id, variant_id, observed_on, price_minor, currency, note)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ListPriceChecks :many
SELECT * FROM price_checks
WHERE (sqlc.narg('variant_id')::uuid IS NULL OR variant_id = sqlc.narg('variant_id'))
  AND (sqlc.narg('store_id')::uuid IS NULL OR store_id = sqlc.narg('store_id'))
ORDER BY observed_on DESC, created_at DESC;

-- name: DeletePriceCheck :execrows
DELETE FROM price_checks WHERE id = $1;

-- Receipt workflow queries (ticket 079).

-- name: GetGroceryReceiptByTransaction :one
SELECT * FROM grocery_receipts WHERE transaction_id = $1;

-- name: UpdateGroceryReceipt :one
UPDATE grocery_receipts SET
    store_id = COALESCE(sqlc.narg('store_id'), store_id),
    purchased_on = COALESCE(sqlc.narg('purchased_on')::date, purchased_on),
    total_minor = COALESCE(sqlc.narg('total_minor')::bigint, total_minor),
    transaction_id = CASE WHEN sqlc.arg('set_transaction')::boolean THEN sqlc.narg('transaction_id') ELSE transaction_id END
WHERE id = sqlc.arg('id')
RETURNING *;

-- name: ListGroceryReceiptSummaries :many
SELECT r.*, s.name AS store_name,
       (SELECT count(*) FROM grocery_receipt_lines l WHERE l.receipt_id = r.id)::bigint AS line_count,
       (SELECT count(*) FROM grocery_receipt_lines l WHERE l.receipt_id = r.id AND l.variant_id IS NULL)::bigint AS unassigned_count,
       (SELECT COALESCE(sum(l.amount_minor), 0) FROM grocery_receipt_lines l WHERE l.receipt_id = r.id)::bigint AS lines_total
FROM grocery_receipts r
JOIN grocery_stores s ON s.id = r.store_id AND s.workspace_id = r.workspace_id
WHERE (sqlc.narg('id')::uuid IS NULL OR r.id = sqlc.narg('id'))
  AND (sqlc.narg('store_id')::uuid IS NULL OR r.store_id = sqlc.narg('store_id'))
  AND (sqlc.narg('from_date')::date IS NULL OR r.purchased_on >= sqlc.narg('from_date'))
  AND (sqlc.narg('to_date')::date IS NULL OR r.purchased_on <= sqlc.narg('to_date'))
  AND (NOT sqlc.arg('unassigned_only')::boolean OR EXISTS (
        SELECT 1 FROM grocery_receipt_lines l WHERE l.receipt_id = r.id AND l.variant_id IS NULL))
ORDER BY r.purchased_on DESC, r.created_at DESC;

-- name: ListGroceryReceiptLinesDetailed :many
SELECT l.*, v.product_id AS product_id, p.name AS product_name, v.brand AS brand, v.pack_label AS pack_label
FROM grocery_receipt_lines l
LEFT JOIN product_variants v ON v.id = l.variant_id AND v.workspace_id = l.workspace_id
LEFT JOIN products p ON p.id = v.product_id AND p.workspace_id = v.workspace_id
WHERE l.receipt_id = $1
ORDER BY l.position;

-- name: GetGroceryReceiptLine :one
SELECT * FROM grocery_receipt_lines WHERE id = $1 AND receipt_id = $2;

-- name: FindGroceryStoreByName :one
SELECT * FROM grocery_stores WHERE lower(name) = lower($1) AND archived_at IS NULL;

-- name: FindGroceryProductByName :one
SELECT * FROM products WHERE lower(name) = lower($1) AND archived_at IS NULL;

-- name: FindProductGroupByName :one
SELECT * FROM product_groups WHERE lower(name) = lower($1);

-- name: FindProductVariant :one
SELECT * FROM product_variants
WHERE product_id = sqlc.arg('product_id') AND lower(brand) = lower(sqlc.arg('brand'))
  AND pack_size IS NOT DISTINCT FROM sqlc.narg('pack_size')::numeric;
