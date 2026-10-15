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
