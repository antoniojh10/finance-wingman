-- name: UpsertBudget :exec
INSERT INTO budgets (category_id, currency, month, amount_minor, created_by)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (workspace_id, category_id, currency, month)
DO UPDATE SET amount_minor = EXCLUDED.amount_minor;

-- name: ListEffectiveBudgets :many
-- The budget in force for the month per category and currency: the latest
-- row on or before it. A null amount means the budget was cleared.
SELECT DISTINCT ON (b.category_id, b.currency)
    b.category_id,
    b.currency,
    b.month AS source_month,
    b.amount_minor,
    c.name AS category_name,
    c.color AS category_color,
    c.archived_at AS category_archived_at,
    cur.minor_units
FROM budgets b
JOIN categories c ON c.id = b.category_id
JOIN currencies cur ON cur.code = b.currency
WHERE b.month <= sqlc.arg('month')::date
ORDER BY b.category_id, b.currency, b.month DESC;

-- name: ListBudgetRecurringExpenses :many
-- Active recurring expenses with the currency and owner of their account.
SELECT
    r.id,
    r.category_id,
    r.amount,
    r.interval_unit,
    r.interval_count,
    r.start_on,
    r.total_payments,
    a.currency,
    c.name AS category_name,
    c.color AS category_color,
    cur.minor_units
FROM recurring_items r
JOIN accounts a ON a.id = r.account_id
JOIN currencies cur ON cur.code = a.currency
LEFT JOIN categories c ON c.id = r.category_id
WHERE r.type = 'expense'
  AND r.status = 'active'
  AND (sqlc.narg('owner_id')::uuid IS NULL OR a.owner_user_id = sqlc.narg('owner_id'))
  AND (NOT sqlc.arg('shared_only')::boolean OR a.owner_user_id IS NULL);

-- name: ListAllBudgets :many
-- Every stored budget row, including cleared ones (null amount), for exports.
SELECT
    b.category_id,
    c.name AS category_name,
    b.currency,
    cur.minor_units,
    b.month,
    b.amount_minor
FROM budgets b
JOIN categories c ON c.id = b.category_id
JOIN currencies cur ON cur.code = b.currency
ORDER BY b.month, c.name, b.currency;
