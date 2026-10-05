-- +goose Up
-- The initial balance is the balance on balance_as_of. Only transactions
-- dated after that day change the balance; earlier or same-day ones are
-- already part of the initial balance. Existing accounts are anchored to
-- their creation date.
ALTER TABLE accounts ADD COLUMN balance_as_of date;
UPDATE accounts SET balance_as_of = created_at::date;
ALTER TABLE accounts ALTER COLUMN balance_as_of SET NOT NULL;
ALTER TABLE accounts ALTER COLUMN balance_as_of SET DEFAULT CURRENT_DATE;
COMMENT ON COLUMN accounts.initial_balance IS 'Balance on balance_as_of, in minor units.';

-- +goose Down
COMMENT ON COLUMN accounts.initial_balance IS NULL;
ALTER TABLE accounts DROP COLUMN balance_as_of;
