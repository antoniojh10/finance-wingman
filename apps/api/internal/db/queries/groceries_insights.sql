-- Read-only analytics over receipts and price observations. Unit prices come
-- from the grocery_unit_prices view (minor units per compare unit).

-- name: GroceryReceiptTotals :many
-- Receipt totals per currency and store for a half-open date range.
SELECT r.currency, r.store_id, s.name AS store_name,
       sum(r.total_minor)::bigint AS total_minor,
       count(*)::bigint AS receipt_count
FROM grocery_receipts r
JOIN grocery_stores s ON s.id = r.store_id AND s.workspace_id = r.workspace_id
WHERE r.purchased_on >= sqlc.arg('from_date') AND r.purchased_on < sqlc.arg('to_date')
  AND (sqlc.narg('currency')::text IS NULL OR r.currency = sqlc.narg('currency'))
GROUP BY r.currency, r.store_id, s.name
ORDER BY r.currency, s.name;

-- name: GroceryLineTotalsByGroup :many
-- Line spend per currency and product group. Lines without a variant are
-- unassigned; lines of a product without group have a null group.
SELECT r.currency,
       g.id AS group_id,
       g.name AS group_name,
       g.default_key AS group_default_key,
       g.color AS group_color,
       g.is_food AS group_is_food,
       (l.variant_id IS NOT NULL)::boolean AS assigned,
       sum(l.amount_minor)::bigint AS total_minor
FROM grocery_receipt_lines l
JOIN grocery_receipts r ON r.id = l.receipt_id AND r.workspace_id = l.workspace_id
LEFT JOIN product_variants v ON v.id = l.variant_id AND v.workspace_id = l.workspace_id
LEFT JOIN products p ON p.id = v.product_id AND p.workspace_id = v.workspace_id
LEFT JOIN product_groups g ON g.id = p.group_id AND g.workspace_id = p.workspace_id
WHERE r.purchased_on >= sqlc.arg('from_date') AND r.purchased_on < sqlc.arg('to_date')
  AND (sqlc.narg('currency')::text IS NULL OR r.currency = sqlc.narg('currency'))
GROUP BY r.currency, g.id, g.name, g.default_key, g.color, g.is_food, g.position, (l.variant_id IS NOT NULL)
ORDER BY r.currency, g.position NULLS LAST, g.name;

-- name: GroceryProductSpend :many
-- Spend and quantity (in compare units) per currency and product.
SELECT r.currency, p.id AS product_id, p.name AS product_name, p.compare_unit,
       sum(l.amount_minor)::bigint AS spend_minor,
       sum(l.quantity * coalesce(v.pack_size, 1))::float8 AS quantity
FROM grocery_receipt_lines l
JOIN grocery_receipts r ON r.id = l.receipt_id AND r.workspace_id = l.workspace_id
JOIN product_variants v ON v.id = l.variant_id AND v.workspace_id = l.workspace_id
JOIN products p ON p.id = v.product_id AND p.workspace_id = v.workspace_id
WHERE r.purchased_on >= sqlc.arg('from_date') AND r.purchased_on < sqlc.arg('to_date')
  AND (sqlc.narg('currency')::text IS NULL OR r.currency = sqlc.narg('currency'))
GROUP BY r.currency, p.id, p.name, p.compare_unit
ORDER BY r.currency, spend_minor DESC, p.name;

-- name: GroceryExpenseCategoryExists :one
-- True when the workspace has an expense category called "Groceries".
SELECT EXISTS (
    SELECT 1 FROM categories WHERE kind = 'expense' AND lower(name) = 'groceries'
);

-- name: GroceryCategoryExpenses :many
-- Expenses of the "Groceries" category per currency in a half-open range.
SELECT a.currency, sum(t.amount)::bigint AS total_minor
FROM transactions t
JOIN accounts a ON a.id = t.account_id AND a.workspace_id = t.workspace_id
JOIN categories c ON c.id = t.category_id AND c.workspace_id = t.workspace_id
WHERE t.type = 'expense' AND c.kind = 'expense' AND lower(c.name) = 'groceries'
  AND t.occurred_on >= sqlc.arg('from_date') AND t.occurred_on < sqlc.arg('to_date')
  AND (sqlc.narg('currency')::text IS NULL OR a.currency = sqlc.narg('currency'))
GROUP BY a.currency;

-- name: GroceryLatestUnitPrices :many
-- Latest observation per (store, variant, currency) inside a date window.
-- Ties on the date prefer receipts over price checks, then the newest row.
-- With a product_id archived products are kept.
SELECT DISTINCT ON (u.store_id, u.variant_id, u.currency)
       u.source, u.store_id, s.name AS store_name, u.variant_id, u.product_id,
       u.currency, u.observed_on,
       u.unit_price_minor::float8 AS unit_price_minor,
       v.brand, v.pack_size, v.pack_label, v.store_id AS variant_store_id,
       p.name AS product_name, p.compare_unit, p.display_multiplier
FROM grocery_unit_prices u
JOIN product_variants v ON v.id = u.variant_id
JOIN products p ON p.id = u.product_id
JOIN grocery_stores s ON s.id = u.store_id
WHERE u.observed_on >= sqlc.arg('since') AND u.observed_on <= sqlc.arg('until')
  AND (sqlc.narg('currency')::text IS NULL OR u.currency = sqlc.narg('currency'))
  AND (sqlc.narg('product_id')::uuid IS NULL OR u.product_id = sqlc.narg('product_id'))
  AND (sqlc.narg('product_id')::uuid IS NOT NULL OR p.archived_at IS NULL)
  AND (NOT sqlc.arg('active_stores_only')::boolean OR s.archived_at IS NULL)
ORDER BY u.store_id, u.variant_id, u.currency, u.observed_on DESC, u.source DESC, u.created_at DESC;

-- name: GroceryBasketUsage :many
-- Quantity (compare units) and spend per currency and product over a
-- half-open date range; the basket divides them by the number of months.
SELECT r.currency, p.id AS product_id, p.name AS product_name, p.compare_unit, p.display_multiplier,
       sum(l.quantity * coalesce(v.pack_size, 1))::float8 AS quantity,
       sum(l.amount_minor)::bigint AS paid_minor
FROM grocery_receipt_lines l
JOIN grocery_receipts r ON r.id = l.receipt_id AND r.workspace_id = l.workspace_id
JOIN product_variants v ON v.id = l.variant_id AND v.workspace_id = l.workspace_id
JOIN products p ON p.id = v.product_id AND p.workspace_id = v.workspace_id
WHERE r.purchased_on >= sqlc.arg('from_date') AND r.purchased_on < sqlc.arg('to_date')
  AND (sqlc.narg('currency')::text IS NULL OR r.currency = sqlc.narg('currency'))
  AND p.archived_at IS NULL
GROUP BY r.currency, p.id, p.name, p.compare_unit, p.display_multiplier
ORDER BY r.currency, p.name;

-- name: GroceryProductPriceHistory :many
-- Average unit price per month for each (variant, store, currency) of a product.
SELECT u.variant_id, u.store_id, u.currency,
       to_char(date_trunc('month', u.observed_on), 'YYYY-MM')::text AS month,
       avg(u.unit_price_minor)::float8 AS avg_unit_price_minor,
       count(*)::bigint AS observations
FROM grocery_unit_prices u
WHERE u.product_id = sqlc.arg('product_id') AND u.observed_on >= sqlc.arg('since') AND u.observed_on <= sqlc.arg('until')
GROUP BY u.variant_id, u.store_id, u.currency, date_trunc('month', u.observed_on)
ORDER BY u.variant_id, u.store_id, u.currency, date_trunc('month', u.observed_on);
