-- +goose Up
CREATE EXTENSION IF NOT EXISTS citext;

-- Shared trigger that keeps updated_at current.
-- +goose StatementBegin
CREATE FUNCTION set_updated_at() RETURNS trigger AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

-- Users who can access the shared workspace. There is no public sign-up:
-- a user must exist here to be able to request a magic link.
CREATE TABLE users (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email       citext NOT NULL UNIQUE,
    name        text NOT NULL DEFAULT '',
    locale      text NOT NULL DEFAULT 'en' CHECK (locale IN ('en', 'es')),
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TRIGGER users_set_updated_at BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ISO 4217 currencies. minor_units is the number of decimal places, used to
-- convert between stored integer amounts and display values.
CREATE TABLE currencies (
    code        char(3) PRIMARY KEY CHECK (code ~ '^[A-Z]{3}$'),
    name        text NOT NULL,
    symbol      text NOT NULL,
    minor_units smallint NOT NULL CHECK (minor_units BETWEEN 0 AND 4)
);

INSERT INTO currencies (code, name, symbol, minor_units) VALUES
    ('ARS', 'Argentine Peso', '$', 2),
    ('AUD', 'Australian Dollar', 'A$', 2),
    ('BRL', 'Brazilian Real', 'R$', 2),
    ('CAD', 'Canadian Dollar', 'CA$', 2),
    ('CHF', 'Swiss Franc', 'CHF', 2),
    ('CLP', 'Chilean Peso', '$', 0),
    ('CNY', 'Chinese Yuan', '¥', 2),
    ('COP', 'Colombian Peso', '$', 2),
    ('CRC', 'Costa Rican Colón', '₡', 2),
    ('DOP', 'Dominican Peso', 'RD$', 2),
    ('EUR', 'Euro', '€', 2),
    ('GBP', 'British Pound', '£', 2),
    ('GTQ', 'Guatemalan Quetzal', 'Q', 2),
    ('INR', 'Indian Rupee', '₹', 2),
    ('JPY', 'Japanese Yen', '¥', 0),
    ('KRW', 'South Korean Won', '₩', 0),
    ('MXN', 'Mexican Peso', '$', 2),
    ('PEN', 'Peruvian Sol', 'S/', 2),
    ('USD', 'US Dollar', '$', 2),
    ('UYU', 'Uruguayan Peso', '$U', 2);

CREATE TABLE accounts (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name            text NOT NULL CHECK (length(trim(name)) > 0),
    type            text NOT NULL CHECK (type IN ('checking', 'savings', 'credit_card', 'cash', 'investment', 'other')),
    currency        char(3) NOT NULL REFERENCES currencies (code),
    -- Balance before the first recorded transaction, in minor units.
    initial_balance bigint NOT NULL DEFAULT 0,
    archived_at     timestamptz,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX accounts_name_active_key ON accounts (lower(name)) WHERE archived_at IS NULL;

CREATE TRIGGER accounts_set_updated_at BEFORE UPDATE ON accounts
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE categories (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name        text NOT NULL CHECK (length(trim(name)) > 0),
    kind        text NOT NULL CHECK (kind IN ('expense', 'income')),
    color       text CHECK (color ~ '^#[0-9a-fA-F]{6}$'),
    icon        text,
    archived_at timestamptz,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX categories_name_kind_active_key ON categories (lower(name), kind) WHERE archived_at IS NULL;

CREATE TRIGGER categories_set_updated_at BEFORE UPDATE ON categories
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- A transaction is an expense, an income, or a transfer between two accounts.
-- Amounts are always positive and expressed in minor units of the account's
-- currency; the type determines the sign when computing balances.
-- Transfers between accounts with different currencies store the amount
-- received by the destination account in destination_amount.
CREATE TABLE transactions (
    id                     uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    type                   text NOT NULL CHECK (type IN ('expense', 'income', 'transfer')),
    account_id             uuid NOT NULL REFERENCES accounts (id) ON DELETE RESTRICT,
    amount                 bigint NOT NULL CHECK (amount > 0),
    destination_account_id uuid REFERENCES accounts (id) ON DELETE RESTRICT,
    destination_amount     bigint CHECK (destination_amount > 0),
    category_id            uuid REFERENCES categories (id) ON DELETE SET NULL,
    description            text NOT NULL DEFAULT '',
    occurred_on            date NOT NULL,
    created_by             uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at             timestamptz NOT NULL DEFAULT now(),
    updated_at             timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT transactions_transfer_fields CHECK (
        (type = 'transfer'
            AND destination_account_id IS NOT NULL
            AND destination_account_id <> account_id
            AND destination_amount IS NOT NULL
            AND category_id IS NULL)
        OR
        (type <> 'transfer'
            AND destination_account_id IS NULL
            AND destination_amount IS NULL)
    )
);

CREATE INDEX transactions_occurred_on_idx ON transactions (occurred_on DESC, created_at DESC);
CREATE INDEX transactions_account_id_idx ON transactions (account_id);
CREATE INDEX transactions_destination_account_id_idx ON transactions (destination_account_id) WHERE destination_account_id IS NOT NULL;
CREATE INDEX transactions_category_id_idx ON transactions (category_id) WHERE category_id IS NOT NULL;

CREATE TRIGGER transactions_set_updated_at BEFORE UPDATE ON transactions
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TABLE transactions;
DROP TABLE categories;
DROP TABLE accounts;
DROP TABLE currencies;
DROP TABLE users;
DROP FUNCTION set_updated_at();
DROP EXTENSION IF EXISTS citext;
