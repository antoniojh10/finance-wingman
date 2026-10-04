-- Balance = initial balance + income - expenses - transfers out + transfers in.

-- name: ListAccounts :many
SELECT
    a.*,
    c.minor_units,
    (a.initial_balance + COALESCE(b.delta, 0))::bigint AS balance
FROM accounts a
JOIN currencies c ON c.code = a.currency
LEFT JOIN LATERAL (
    SELECT sum(
        CASE
            WHEN t.account_id = a.id AND t.type = 'income' THEN t.amount
            WHEN t.account_id = a.id THEN -t.amount
            ELSE t.destination_amount
        END
    ) AS delta
    FROM transactions t
    WHERE t.account_id = a.id OR t.destination_account_id = a.id
) b ON true
WHERE sqlc.arg('include_archived')::boolean OR a.archived_at IS NULL
ORDER BY a.archived_at IS NOT NULL, lower(a.name);

-- name: GetAccount :one
SELECT
    a.*,
    c.minor_units,
    (a.initial_balance + COALESCE(b.delta, 0))::bigint AS balance
FROM accounts a
JOIN currencies c ON c.code = a.currency
LEFT JOIN LATERAL (
    SELECT sum(
        CASE
            WHEN t.account_id = a.id AND t.type = 'income' THEN t.amount
            WHEN t.account_id = a.id THEN -t.amount
            ELSE t.destination_amount
        END
    ) AS delta
    FROM transactions t
    WHERE t.account_id = a.id OR t.destination_account_id = a.id
) b ON true
WHERE a.id = $1;

-- name: CreateAccount :one
INSERT INTO accounts (name, type, currency, initial_balance)
VALUES ($1, $2, $3, $4)
RETURNING id;

-- name: UpdateAccount :exec
UPDATE accounts SET
    name = COALESCE(sqlc.narg('name'), name),
    type = COALESCE(sqlc.narg('type'), type),
    initial_balance = COALESCE(sqlc.narg('initial_balance'), initial_balance),
    archived_at = CASE
        WHEN sqlc.narg('archived')::boolean IS NULL THEN archived_at
        WHEN sqlc.narg('archived')::boolean THEN COALESCE(archived_at, now())
        ELSE NULL
    END
WHERE id = sqlc.arg('id');

-- name: CountAccountTransactions :one
SELECT count(*) FROM transactions WHERE account_id = $1 OR destination_account_id = $1;

-- name: DeleteAccount :execrows
DELETE FROM accounts WHERE id = $1;
