-- +goose Up
-- Recurring items: subscriptions, bills, installments and recurring income.
-- They describe a schedule only; payments are registered manually as
-- regular transactions. Retired items are kept with status 'cancelled'.
CREATE TABLE recurring_items (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name           text NOT NULL CHECK (btrim(name) <> ''),
    type           text NOT NULL CHECK (type IN ('expense', 'income')),
    account_id     uuid NOT NULL REFERENCES accounts (id) ON DELETE RESTRICT,
    category_id    uuid REFERENCES categories (id) ON DELETE SET NULL,
    amount         bigint NOT NULL CHECK (amount > 0),
    notes          text NOT NULL DEFAULT '',
    interval_unit  text NOT NULL CHECK (interval_unit IN ('week', 'month', 'year')),
    interval_count integer NOT NULL DEFAULT 1 CHECK (interval_count >= 1),
    start_on       date NOT NULL,
    total_payments integer CHECK (total_payments >= 1),
    status         text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'paused', 'cancelled')),
    created_by     uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX recurring_items_account_id_idx ON recurring_items (account_id);
CREATE INDEX recurring_items_status_idx ON recurring_items (status);

CREATE TRIGGER recurring_items_set_updated_at BEFORE UPDATE ON recurring_items
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TABLE recurring_items;
