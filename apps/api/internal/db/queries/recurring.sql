-- name: ListRecurringItems :many
SELECT
    r.*,
    a.name AS account_name,
    a.currency,
    a.archived_at AS account_archived_at,
    cur.minor_units,
    c.name AS category_name
FROM recurring_items r
JOIN accounts a ON a.id = r.account_id
JOIN currencies cur ON cur.code = a.currency
LEFT JOIN categories c ON c.id = r.category_id
WHERE (sqlc.narg('status')::text IS NULL OR r.status = sqlc.narg('status'))
  AND (sqlc.narg('type')::text IS NULL OR r.type = sqlc.narg('type'))
ORDER BY r.status <> 'active', lower(r.name), r.created_at;

-- name: GetRecurringItem :one
SELECT
    r.*,
    a.name AS account_name,
    a.currency,
    a.archived_at AS account_archived_at,
    cur.minor_units,
    c.name AS category_name
FROM recurring_items r
JOIN accounts a ON a.id = r.account_id
JOIN currencies cur ON cur.code = a.currency
LEFT JOIN categories c ON c.id = r.category_id
WHERE r.id = $1;

-- name: CreateRecurringItem :one
INSERT INTO recurring_items (
    name, type, account_id, category_id, amount, notes,
    interval_unit, interval_count, start_on, total_payments, created_by
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING id;

-- name: UpdateRecurringItem :exec
UPDATE recurring_items SET
    name = $2,
    category_id = $3,
    amount = $4,
    notes = $5,
    interval_unit = $6,
    interval_count = $7,
    start_on = $8,
    total_payments = $9,
    status = $10
WHERE id = $1;

-- name: ListRecurringPaidPeriods :many
SELECT DISTINCT recurring_id, recurring_due_on
FROM transactions
WHERE recurring_id = ANY(sqlc.arg('item_ids')::uuid[]);

-- name: ListRecurringLastPayments :many
SELECT DISTINCT ON (recurring_id)
    recurring_id, id, occurred_on, amount, recurring_due_on
FROM transactions
WHERE recurring_id = ANY(sqlc.arg('item_ids')::uuid[])
ORDER BY recurring_id, occurred_on DESC, created_at DESC, id;

-- name: FindOpenRecurringItemByName :one
SELECT id, name, status FROM recurring_items
WHERE lower(name) = lower($1) AND status <> 'cancelled' AND id <> $2
LIMIT 1;
