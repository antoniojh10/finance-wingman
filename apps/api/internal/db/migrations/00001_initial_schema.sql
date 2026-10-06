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
    initial_balance bigint NOT NULL DEFAULT 0,
    archived_at     timestamptz,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    -- Only transactions dated after this day change the balance; earlier or
    -- same-day ones are already part of initial_balance.
    balance_as_of   date NOT NULL DEFAULT CURRENT_DATE
);

COMMENT ON COLUMN accounts.initial_balance IS 'Balance on balance_as_of, in minor units.';

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

-- Names are unique (case-insensitive) among non-cancelled recurring items,
-- so a cancelled name can be reused for a new item.
CREATE UNIQUE INDEX recurring_items_active_name_key
    ON recurring_items (lower(name)) WHERE status <> 'cancelled';

CREATE TRIGGER recurring_items_set_updated_at BEFORE UPDATE ON recurring_items
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
    -- The recurring item this transaction pays and the due date (period) it
    -- settles. Both are set together or both are null.
    recurring_id           uuid REFERENCES recurring_items (id) ON DELETE SET NULL,
    recurring_due_on       date,

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
    ),
    CONSTRAINT transactions_recurring_link_check
        CHECK ((recurring_id IS NULL) = (recurring_due_on IS NULL))
);

CREATE INDEX transactions_occurred_on_idx ON transactions (occurred_on DESC, created_at DESC);
CREATE INDEX transactions_account_id_idx ON transactions (account_id);
CREATE INDEX transactions_destination_account_id_idx ON transactions (destination_account_id) WHERE destination_account_id IS NOT NULL;
CREATE INDEX transactions_category_id_idx ON transactions (category_id) WHERE category_id IS NOT NULL;
CREATE INDEX transactions_recurring_idx ON transactions (recurring_id, recurring_due_on);

CREATE TRIGGER transactions_set_updated_at BEFORE UPDATE ON transactions
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose StatementBegin
-- The foreign key action would null recurring_id alone and violate the
-- link check on transactions, so clear both columns before the row is
-- deleted.
CREATE FUNCTION recurring_items_unlink_transactions() RETURNS trigger AS $$
BEGIN
    UPDATE transactions SET recurring_id = NULL, recurring_due_on = NULL WHERE recurring_id = OLD.id;
    RETURN OLD;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER recurring_items_unlink_transactions BEFORE DELETE ON recurring_items
    FOR EACH ROW EXECUTE FUNCTION recurring_items_unlink_transactions();

-- Detected recurring-item suggestions the user dismissed, so they do not
-- reappear. A suggestion is identified by its account, type and normalized
-- description, whatever frequency or amount it is detected with.
CREATE TABLE recurring_dismissed_suggestions (
    account_id   uuid NOT NULL REFERENCES accounts (id) ON DELETE CASCADE,
    type         text NOT NULL CHECK (type IN ('expense', 'income')),
    description  text NOT NULL CHECK (description <> ''),
    dismissed_by uuid REFERENCES users (id) ON DELETE SET NULL,
    dismissed_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (account_id, type, description)
);

-- A login challenge is created when a user requests a magic link. It can be
-- redeemed once, either through the link token or the short numeric code
-- included in the same email.
CREATE TABLE login_challenges (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_hash  bytea NOT NULL UNIQUE,
    code_hash   bytea NOT NULL,
    attempts    integer NOT NULL DEFAULT 0,
    expires_at  timestamptz NOT NULL,
    consumed_at timestamptz,
    created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX login_challenges_user_id_idx ON login_challenges (user_id, created_at DESC);

-- OAuth clients registered dynamically (RFC 7591) by MCP hosts such as
-- Claude or ChatGPT.
CREATE TABLE oauth_clients (
    id                         text PRIMARY KEY,
    secret_hash                bytea,
    name                       text NOT NULL DEFAULT '',
    redirect_uris              text[] NOT NULL CHECK (cardinality(redirect_uris) > 0),
    token_endpoint_auth_method text NOT NULL CHECK (token_endpoint_auth_method IN ('none', 'client_secret_post', 'client_secret_basic')),
    created_at                 timestamptz NOT NULL DEFAULT now()
);

-- Sessions are opaque bearer tokens; only their SHA-256 hash is stored.
-- Access tokens issued through OAuth are regular sessions linked to their
-- client and refresh token family.
CREATE TABLE sessions (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_hash      bytea NOT NULL UNIQUE,
    client          text NOT NULL DEFAULT 'web',
    expires_at      timestamptz NOT NULL,
    last_used_at    timestamptz NOT NULL DEFAULT now(),
    created_at      timestamptz NOT NULL DEFAULT now(),
    oauth_client_id text REFERENCES oauth_clients (id) ON DELETE CASCADE,
    oauth_family_id uuid
);

CREATE INDEX sessions_user_id_idx ON sessions (user_id);
CREATE INDEX sessions_oauth_family_idx ON sessions (oauth_family_id) WHERE oauth_family_id IS NOT NULL;

-- An authorization request in progress: created when the user lands on the
-- authorize page and completed once they enter the emailed code.
CREATE TABLE oauth_authorization_requests (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id      text NOT NULL REFERENCES oauth_clients (id) ON DELETE CASCADE,
    redirect_uri   text NOT NULL,
    code_challenge text NOT NULL,
    state          text NOT NULL DEFAULT '',
    scope          text NOT NULL DEFAULT '',
    resource       text NOT NULL DEFAULT '',
    email          citext,
    expires_at     timestamptz NOT NULL,
    created_at     timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE oauth_authorization_codes (
    code_hash      bytea PRIMARY KEY,
    client_id      text NOT NULL REFERENCES oauth_clients (id) ON DELETE CASCADE,
    user_id        uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    redirect_uri   text NOT NULL,
    code_challenge text NOT NULL,
    scope          text NOT NULL DEFAULT '',
    expires_at     timestamptz NOT NULL,
    used_at        timestamptz,
    created_at     timestamptz NOT NULL DEFAULT now()
);

-- Refresh tokens rotate on every use. All tokens descending from the same
-- authorization share a family so reuse of a rotated token revokes them all.
CREATE TABLE oauth_refresh_tokens (
    token_hash bytea PRIMARY KEY,
    family_id  uuid NOT NULL,
    client_id  text NOT NULL REFERENCES oauth_clients (id) ON DELETE CASCADE,
    user_id    uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    scope      text NOT NULL DEFAULT '',
    expires_at timestamptz NOT NULL,
    revoked_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX oauth_refresh_tokens_family_idx ON oauth_refresh_tokens (family_id);

-- +goose Down
DROP TABLE oauth_refresh_tokens;
DROP TABLE oauth_authorization_codes;
DROP TABLE oauth_authorization_requests;
DROP TABLE sessions;
DROP TABLE oauth_clients;
DROP TABLE login_challenges;
DROP TABLE recurring_dismissed_suggestions;
DROP TABLE transactions;
DROP TABLE recurring_items;
DROP FUNCTION recurring_items_unlink_transactions();
DROP TABLE categories;
DROP TABLE accounts;
DROP TABLE currencies;
DROP TABLE users;
DROP FUNCTION set_updated_at();
DROP EXTENSION IF EXISTS citext;
