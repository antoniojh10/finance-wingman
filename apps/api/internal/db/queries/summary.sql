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
GROUP BY a.currency, t.type, t.category_id, c.name, c.color
ORDER BY a.currency, t.type, total DESC;
