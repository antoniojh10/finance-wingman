-- +goose Up

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

-- Sessions are opaque bearer tokens; only their SHA-256 hash is stored.
CREATE TABLE sessions (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_hash   bytea NOT NULL UNIQUE,
    client       text NOT NULL DEFAULT 'web',
    expires_at   timestamptz NOT NULL,
    last_used_at timestamptz NOT NULL DEFAULT now(),
    created_at   timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX sessions_user_id_idx ON sessions (user_id);

-- +goose Down
DROP TABLE sessions;
DROP TABLE login_challenges;
