-- name: CreateTransaction :one
INSERT INTO transactions (
    type, account_id, amount, destination_account_id, destination_amount,
    category_id, description, occurred_on, created_by
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9
)
RETURNING id;

-- name: GetTransaction :one
SELECT
    t.*,
    a.name AS account_name,
    a.currency AS currency,
    cur.minor_units AS minor_units,
    da.name AS destination_account_name,
    da.currency AS destination_currency,
    dcur.minor_units AS destination_minor_units,
    c.name AS category_name,
    u.name AS created_by_name,
    u.email AS created_by_email,
    a.owner_user_id AS account_owner_id,
    ao.name AS account_owner_name,
    ao.email AS account_owner_email,
    da.owner_user_id AS destination_account_owner_id,
    dao.name AS destination_account_owner_name,
    dao.email AS destination_account_owner_email
FROM transactions t
JOIN accounts a ON a.id = t.account_id
JOIN currencies cur ON cur.code = a.currency
LEFT JOIN accounts da ON da.id = t.destination_account_id
LEFT JOIN currencies dcur ON dcur.code = da.currency
LEFT JOIN categories c ON c.id = t.category_id
LEFT JOIN users u ON u.id = t.created_by
LEFT JOIN users ao ON ao.id = a.owner_user_id
LEFT JOIN users dao ON dao.id = da.owner_user_id
WHERE t.id = $1;

-- name: ListTransactions :many
SELECT
    t.*,
    a.name AS account_name,
    a.currency AS currency,
    cur.minor_units AS minor_units,
    da.name AS destination_account_name,
    da.currency AS destination_currency,
    dcur.minor_units AS destination_minor_units,
    c.name AS category_name,
    u.name AS created_by_name,
    u.email AS created_by_email,
    a.owner_user_id AS account_owner_id,
    ao.name AS account_owner_name,
    ao.email AS account_owner_email,
    da.owner_user_id AS destination_account_owner_id,
    dao.name AS destination_account_owner_name,
    dao.email AS destination_account_owner_email,
    count(*) OVER () AS total_count
FROM transactions t
JOIN accounts a ON a.id = t.account_id
JOIN currencies cur ON cur.code = a.currency
LEFT JOIN accounts da ON da.id = t.destination_account_id
LEFT JOIN currencies dcur ON dcur.code = da.currency
LEFT JOIN categories c ON c.id = t.category_id
LEFT JOIN users u ON u.id = t.created_by
LEFT JOIN users ao ON ao.id = a.owner_user_id
LEFT JOIN users dao ON dao.id = da.owner_user_id
WHERE (sqlc.narg('account_id')::uuid IS NULL
        OR t.account_id = sqlc.narg('account_id')
        OR t.destination_account_id = sqlc.narg('account_id'))
  AND (sqlc.narg('category_id')::uuid IS NULL OR t.category_id = sqlc.narg('category_id'))
  AND (sqlc.narg('type')::text IS NULL OR t.type = sqlc.narg('type'))
  AND (sqlc.narg('from_date')::date IS NULL OR t.occurred_on >= sqlc.narg('from_date'))
  AND (sqlc.narg('to_date')::date IS NULL OR t.occurred_on <= sqlc.narg('to_date'))
  AND (sqlc.narg('search')::text IS NULL OR t.description ILIKE '%' || sqlc.narg('search') || '%')
  -- Owner filter: a transfer matches when either side is in the owner's accounts.
  AND (sqlc.narg('owner_id')::uuid IS NULL
        OR a.owner_user_id = sqlc.narg('owner_id')
        OR da.owner_user_id = sqlc.narg('owner_id'))
  AND (NOT sqlc.arg('shared_only')::boolean
        OR a.owner_user_id IS NULL
        OR (da.id IS NOT NULL AND da.owner_user_id IS NULL))
ORDER BY t.occurred_on DESC, t.created_at DESC, t.id
LIMIT sqlc.arg('row_limit') OFFSET sqlc.arg('row_offset');

-- name: UpdateTransaction :exec
UPDATE transactions SET
    type = $2,
    account_id = $3,
    amount = $4,
    destination_account_id = $5,
    destination_amount = $6,
    category_id = $7,
    description = $8,
    occurred_on = $9
WHERE id = $1;

-- name: GetTransactionRecord :one
SELECT * FROM transactions WHERE id = $1;

-- name: DeleteTransaction :execrows
DELETE FROM transactions WHERE id = $1;

-- name: LinkTransactionToRecurring :exec
UPDATE transactions SET recurring_id = $2, recurring_due_on = $3 WHERE id = $1;

-- name: UnlinkTransactionFromRecurring :exec
UPDATE transactions SET recurring_id = NULL, recurring_due_on = NULL WHERE id = $1;
