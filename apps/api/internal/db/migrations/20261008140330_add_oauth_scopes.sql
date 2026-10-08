-- +goose Up
-- OAuth access tokens (sessions of a grant) carry the scope they were
-- issued with: "finance:read", or "finance:read finance:write". Sign-in
-- sessions have no scope and full access. OAuth sessions and grants issued
-- before scopes existed have "" or the legacy scope "finance", which keep
-- full access.
ALTER TABLE sessions ADD COLUMN scope text NOT NULL DEFAULT '';

-- +goose Down
-- The previous version cannot restrict a connection to reading: it would
-- refresh a read-only connection into full access. Disconnect read-only
-- connections (and drop pending read-only authorizations) instead of
-- widening what the user allowed; they can connect again after rollback.
UPDATE oauth_grants SET revoked_at = now()
WHERE scope = 'finance:read' AND revoked_at IS NULL;
UPDATE oauth_refresh_tokens SET revoked_at = now()
WHERE scope = 'finance:read' AND revoked_at IS NULL;
DELETE FROM sessions WHERE scope = 'finance:read';
DELETE FROM oauth_authorization_codes WHERE scope = 'finance:read';
DELETE FROM oauth_authorization_requests WHERE scope = 'finance:read';
ALTER TABLE sessions DROP COLUMN scope;
