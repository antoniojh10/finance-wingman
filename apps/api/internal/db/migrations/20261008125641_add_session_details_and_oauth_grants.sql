-- +goose Up
-- The browser a web session was opened from, shown in the user's list of
-- sessions. Sessions opened before this migration have none.
ALTER TABLE sessions ADD COLUMN user_agent text NOT NULL DEFAULT '';

-- An OAuth grant is one authorization of a client by a user, acting on a
-- workspace: a connected app. Its id is the family id shared by the refresh
-- tokens (and access token sessions) issued for it. Revoking the grant
-- disconnects the app; refreshes lock the grant row first, so a refresh in
-- flight cannot issue tokens after the grant is revoked.
CREATE TABLE oauth_grants (
    id           uuid PRIMARY KEY,
    client_id    text NOT NULL REFERENCES oauth_clients (id) ON DELETE CASCADE,
    user_id      uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    workspace_id uuid REFERENCES workspaces (id) ON DELETE CASCADE,
    scope        text NOT NULL DEFAULT '',
    created_at   timestamptz NOT NULL DEFAULT now(),
    last_used_at timestamptz NOT NULL DEFAULT now(),
    revoked_at   timestamptz
);

CREATE INDEX oauth_grants_user_id_idx ON oauth_grants (user_id);

-- Existing authorizations become grants: connected when their first refresh
-- token was issued, last used when the latest one was, and revoked once no
-- token of the family is active.
INSERT INTO oauth_grants (id, client_id, user_id, workspace_id, scope, created_at, last_used_at, revoked_at)
SELECT
    family_id,
    (array_agg(client_id ORDER BY created_at DESC))[1],
    (array_agg(user_id ORDER BY created_at DESC))[1],
    (array_agg(workspace_id ORDER BY created_at DESC))[1],
    (array_agg(scope ORDER BY created_at DESC))[1],
    min(created_at),
    max(created_at),
    CASE WHEN bool_or(revoked_at IS NULL) THEN NULL ELSE max(revoked_at) END
FROM oauth_refresh_tokens
GROUP BY family_id;

ALTER TABLE oauth_refresh_tokens
    ADD CONSTRAINT oauth_refresh_tokens_family_fkey FOREIGN KEY (family_id)
        REFERENCES oauth_grants (id) ON DELETE CASCADE;

-- +goose Down
ALTER TABLE oauth_refresh_tokens DROP CONSTRAINT oauth_refresh_tokens_family_fkey;
DROP TABLE oauth_grants;
ALTER TABLE sessions DROP COLUMN user_agent;
