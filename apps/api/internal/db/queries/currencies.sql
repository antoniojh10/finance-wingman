-- name: ListCurrencies :many
SELECT * FROM currencies ORDER BY code;

-- name: GetCurrency :one
SELECT * FROM currencies WHERE code = $1;
