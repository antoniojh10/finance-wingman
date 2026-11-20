-- Income and expense totals per currency and category for a date range.
-- Transfers are excluded: they move money between accounts but are neither
-- income nor expense.

-- name: SummaryByCategory :many
SELECT
    a.currency,
    t.type,
    t.category_id,
    c.name AS category_name,
    c.color AS category_color,
    sum(t.amount)::bigint AS total,
    count(*) AS transaction_count
FROM transactions t
JOIN accounts a ON a.id = t.account_id
LEFT JOIN categories c ON c.id = t.category_id
WHERE t.type IN ('income', 'expense')
  AND t.occurred_on BETWEEN sqlc.arg('from_date') AND sqlc.arg('to_date')
  AND (sqlc.narg('owner_id')::uuid IS NULL OR a.owner_user_id = sqlc.narg('owner_id'))
  AND (NOT sqlc.arg('shared_only')::boolean OR a.owner_user_id IS NULL)
GROUP BY a.currency, t.type, t.category_id, c.name, c.color
ORDER BY a.currency, t.type, total DESC;

-- name: SummaryMonthlyExpenses :many
-- Expenses per currency, category and calendar month in a date range.
-- Uncategorized expenses come back with a null category. Income and
-- transfers are not expenses.
SELECT
    a.currency,
    t.category_id,
    c.name AS category_name,
    c.color AS category_color,
    c.archived_at AS category_archived_at,
    date_trunc('month', t.occurred_on)::date AS month,
    sum(t.amount)::bigint AS total
FROM transactions t
JOIN accounts a ON a.id = t.account_id
LEFT JOIN categories c ON c.id = t.category_id
WHERE t.type = 'expense'
  AND t.occurred_on BETWEEN sqlc.arg('from_date') AND sqlc.arg('to_date')
  AND (sqlc.narg('owner_id')::uuid IS NULL OR a.owner_user_id = sqlc.narg('owner_id'))
  AND (NOT sqlc.arg('shared_only')::boolean OR a.owner_user_id IS NULL)
GROUP BY a.currency, t.category_id, c.name, c.color, c.archived_at, date_trunc('month', t.occurred_on);

-- name: DailyOpeningBalances :many
-- Balance of every account matching the filters at the end of as_of_date:
-- initial_balance - deltas(<= balance_as_of) + deltas(<= as_of_date), so the
-- result stays consistent with the anchor rule of ListAccounts for any day.
-- Per-side delta: income +amount, expense and transfer out -amount,
-- transfer in +destination_amount.
WITH sides AS (
    SELECT t.account_id, t.occurred_on,
           CASE WHEN t.type = 'income' THEN t.amount ELSE -t.amount END AS delta
    FROM transactions t
    UNION ALL
    SELECT t.destination_account_id, t.occurred_on, t.destination_amount
    FROM transactions t
    WHERE t.destination_account_id IS NOT NULL
)
SELECT
    a.id AS account_id,
    (a.initial_balance
        + COALESCE(sum(s.delta) FILTER (WHERE s.occurred_on <= sqlc.arg('as_of_date')::date), 0)
        - COALESCE(sum(s.delta) FILTER (WHERE s.occurred_on <= a.balance_as_of), 0))::bigint AS balance
FROM accounts a
LEFT JOIN sides s ON s.account_id = a.id
    AND s.occurred_on <= GREATEST(sqlc.arg('as_of_date')::date, a.balance_as_of)
WHERE a.archived_at IS NULL
  AND (sqlc.narg('owner_id')::uuid IS NULL OR a.owner_user_id = sqlc.narg('owner_id'))
  AND (NOT sqlc.arg('shared_only')::boolean OR a.owner_user_id IS NULL)
GROUP BY a.id;

-- name: DailyAccountDeltas :many
-- Net movement per account and day in a date range, with the income and
-- expense (transfers excluded) of that day.
WITH sides AS (
    SELECT t.account_id, t.occurred_on, t.type,
           CASE WHEN t.type = 'income' THEN t.amount ELSE -t.amount END AS delta
    FROM transactions t
    UNION ALL
    SELECT t.destination_account_id, t.occurred_on, 'transfer_in', t.destination_amount
    FROM transactions t
    WHERE t.destination_account_id IS NOT NULL
)
SELECT
    a.id AS account_id,
    s.occurred_on AS day,
    sum(s.delta)::bigint AS delta,
    COALESCE(sum(s.delta) FILTER (WHERE s.type = 'income'), 0)::bigint AS income,
    COALESCE(-sum(s.delta) FILTER (WHERE s.type = 'expense'), 0)::bigint AS expense
FROM accounts a
JOIN sides s ON s.account_id = a.id
WHERE a.archived_at IS NULL
  AND s.occurred_on >= sqlc.arg('from_date')::date AND s.occurred_on <= sqlc.arg('to_date')::date
  AND (sqlc.narg('owner_id')::uuid IS NULL OR a.owner_user_id = sqlc.narg('owner_id'))
  AND (NOT sqlc.arg('shared_only')::boolean OR a.owner_user_id IS NULL)
GROUP BY a.id, s.occurred_on;

-- name: DailyBaselineExpenses :many
-- Expenses not linked to a recurring item per account in a date range, used
-- to estimate the average daily spending.
SELECT
    a.id AS account_id,
    COALESCE(sum(t.amount), 0)::bigint AS total
FROM accounts a
JOIN transactions t ON t.account_id = a.id
WHERE a.archived_at IS NULL
  AND t.type = 'expense'
  AND t.recurring_id IS NULL
  AND t.occurred_on >= sqlc.arg('from_date')::date AND t.occurred_on <= sqlc.arg('to_date')::date
  AND (sqlc.narg('owner_id')::uuid IS NULL OR a.owner_user_id = sqlc.narg('owner_id'))
  AND (NOT sqlc.arg('shared_only')::boolean OR a.owner_user_id IS NULL)
GROUP BY a.id;
