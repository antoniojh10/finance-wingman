-- Balance = initial balance + income - expenses - transfers out + transfers in,
-- counting only transactions dated after the account's balance_as_of day
-- (the initial balance already includes everything up to and including it).
-- Transfers apply the rule per side, using each account's own anchor.

-- name: ListAccounts :many
SELECT
    a.*,
    c.minor_units,
    (a.initial_balance + COALESCE(b.delta, 0))::bigint AS balance,
    o.name AS owner_name,
    o.email AS owner_email
FROM accounts a
JOIN currencies c ON c.code = a.currency
LEFT JOIN users o ON o.id = a.owner_user_id
LEFT JOIN LATERAL (
    SELECT sum(
        CASE
            WHEN t.account_id = a.id AND t.type = 'income' THEN t.amount
            WHEN t.account_id = a.id THEN -t.amount
            ELSE t.destination_amount
        END
    ) AS delta
    FROM transactions t
    WHERE (t.account_id = a.id OR t.destination_account_id = a.id)
      AND t.occurred_on > a.balance_as_of
) b ON true
WHERE (sqlc.arg('include_archived')::boolean OR a.archived_at IS NULL)
  AND (sqlc.narg('owner_id')::uuid IS NULL OR a.owner_user_id = sqlc.narg('owner_id'))
  AND (NOT sqlc.arg('shared_only')::boolean OR a.owner_user_id IS NULL)
ORDER BY a.archived_at IS NOT NULL, lower(a.name), o.name NULLS FIRST;

-- name: GetAccount :one
SELECT
    a.*,
    c.minor_units,
    (a.initial_balance + COALESCE(b.delta, 0))::bigint AS balance,
    o.name AS owner_name,
    o.email AS owner_email
FROM accounts a
JOIN currencies c ON c.code = a.currency
LEFT JOIN users o ON o.id = a.owner_user_id
LEFT JOIN LATERAL (
    SELECT sum(
        CASE
            WHEN t.account_id = a.id AND t.type = 'income' THEN t.amount
            WHEN t.account_id = a.id THEN -t.amount
            ELSE t.destination_amount
        END
    ) AS delta
    FROM transactions t
    WHERE (t.account_id = a.id OR t.destination_account_id = a.id)
      AND t.occurred_on > a.balance_as_of
) b ON true
WHERE a.id = $1;

-- name: CreateAccount :one
INSERT INTO accounts (name, type, currency, initial_balance, balance_as_of, owner_user_id)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id;

-- name: UpdateAccount :exec
UPDATE accounts SET
    name = COALESCE(sqlc.narg('name'), name),
    type = COALESCE(sqlc.narg('type'), type),
    initial_balance = COALESCE(sqlc.narg('initial_balance'), initial_balance),
    balance_as_of = COALESCE(sqlc.narg('balance_as_of'), balance_as_of),
    owner_user_id = CASE WHEN sqlc.arg('set_owner')::boolean THEN sqlc.narg('owner_user_id')::uuid ELSE owner_user_id END,
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

-- Members of the current workspace, who can own accounts.
-- name: ListAccountOwners :many
SELECT u.id, u.name, u.email
FROM workspace_members m
JOIN users u ON u.id = m.user_id
WHERE m.workspace_id = current_workspace_id()
ORDER BY lower(u.name), u.email;
