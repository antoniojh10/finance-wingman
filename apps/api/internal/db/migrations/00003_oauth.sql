-- +goose Up

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

-- Access tokens issued through OAuth are regular sessions linked to their
-- client and refresh token family.
ALTER TABLE sessions
    ADD COLUMN oauth_client_id text REFERENCES oauth_clients (id) ON DELETE CASCADE,
    ADD COLUMN oauth_family_id uuid;

CREATE INDEX sessions_oauth_family_idx ON sessions (oauth_family_id) WHERE oauth_family_id IS NOT NULL;

-- +goose Down
ALTER TABLE sessions DROP COLUMN oauth_family_id, DROP COLUMN oauth_client_id;
DROP TABLE oauth_refresh_tokens;
DROP TABLE oauth_authorization_codes;
DROP TABLE oauth_authorization_requests;
DROP TABLE oauth_clients;
