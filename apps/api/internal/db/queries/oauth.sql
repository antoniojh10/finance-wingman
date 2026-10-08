-- name: CreateOAuthClient :one
INSERT INTO oauth_clients (id, secret_hash, name, redirect_uris, token_endpoint_auth_method)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetOAuthClient :one
SELECT * FROM oauth_clients WHERE id = $1;

-- name: CountRecentOAuthClients :one
SELECT count(*) FROM oauth_clients WHERE created_at > $1;

-- name: CreateAuthorizationRequest :one
INSERT INTO oauth_authorization_requests (client_id, redirect_uri, code_challenge, state, scope, resource, expires_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetAuthorizationRequest :one
SELECT r.*, c.name AS client_name
FROM oauth_authorization_requests r
JOIN oauth_clients c ON c.id = r.client_id
WHERE r.id = $1;

-- name: SetAuthorizationRequestEmail :exec
UPDATE oauth_authorization_requests SET email = $2 WHERE id = $1;

-- name: SetAuthorizationRequestUser :exec
UPDATE oauth_authorization_requests SET user_id = $2 WHERE id = $1;

-- name: DeleteAuthorizationRequest :exec
DELETE FROM oauth_authorization_requests WHERE id = $1;

-- name: CreateAuthorizationCode :exec
INSERT INTO oauth_authorization_codes (code_hash, client_id, user_id, redirect_uri, code_challenge, scope, workspace_id, expires_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8);

-- Marks the code as used and returns it, only if it was unused.
-- name: UseAuthorizationCode :one
UPDATE oauth_authorization_codes SET used_at = now()
WHERE code_hash = $1 AND used_at IS NULL
RETURNING *;

-- name: GetAuthorizationCode :one
SELECT * FROM oauth_authorization_codes WHERE code_hash = $1;

-- name: CreateOAuthGrant :exec
INSERT INTO oauth_grants (id, client_id, user_id, workspace_id, scope)
VALUES ($1, $2, $3, $4, $5);

-- Locks the grant so its tokens are issued and revoked one flow at a time:
-- every flow that issues or revokes a grant's tokens takes this lock first.
-- name: LockOAuthGrant :one
SELECT * FROM oauth_grants WHERE id = $1 FOR UPDATE;

-- name: TouchOAuthGrant :exec
UPDATE oauth_grants SET last_used_at = now() WHERE id = $1;

-- name: RevokeOAuthGrant :exec
UPDATE oauth_grants SET revoked_at = now() WHERE id = $1 AND revoked_at IS NULL;

-- The user's connected apps: grants with a refresh token still usable.
-- Last use is the latest refresh or call made with one of its access tokens.
-- name: ListUserOAuthGrants :many
SELECT g.id, c.name AS client_name, g.workspace_id, w.name AS workspace_name, g.scope, g.created_at,
    greatest(g.last_used_at, (
        SELECT max(s.last_used_at) FROM sessions s WHERE s.oauth_family_id = g.id
    ))::timestamptz AS last_used_at
FROM oauth_grants g
JOIN oauth_clients c ON c.id = g.client_id
LEFT JOIN workspaces w ON w.id = g.workspace_id
WHERE g.user_id = sqlc.arg('user_id')
  AND g.revoked_at IS NULL
  AND EXISTS (
    SELECT 1 FROM oauth_refresh_tokens t
    WHERE t.family_id = g.id AND t.revoked_at IS NULL AND t.expires_at > sqlc.arg('now')
  )
ORDER BY last_used_at DESC, g.created_at DESC;

-- name: CreateRefreshToken :exec
INSERT INTO oauth_refresh_tokens (token_hash, family_id, client_id, user_id, scope, workspace_id, expires_at)
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- Revokes the token and returns it, only if it was still active.
-- name: RotateRefreshToken :one
UPDATE oauth_refresh_tokens SET revoked_at = now()
WHERE token_hash = $1 AND revoked_at IS NULL
RETURNING *;

-- name: GetRefreshToken :one
SELECT * FROM oauth_refresh_tokens WHERE token_hash = $1;

-- name: RevokeRefreshTokenFamily :exec
UPDATE oauth_refresh_tokens SET revoked_at = now()
WHERE family_id = $1 AND revoked_at IS NULL;

-- name: DeleteFamilySessions :exec
DELETE FROM sessions WHERE oauth_family_id = $1;

-- name: CreateOAuthSession :one
INSERT INTO sessions (user_id, token_hash, client, expires_at, oauth_client_id, oauth_family_id, workspace_id)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id;

-- name: GetSessionFamily :one
SELECT oauth_family_id FROM sessions WHERE token_hash = $1;

-- name: DeleteExpiredOAuthRecords :exec
WITH expired_requests AS (
    DELETE FROM oauth_authorization_requests WHERE oauth_authorization_requests.expires_at < $1
), expired_codes AS (
    DELETE FROM oauth_authorization_codes WHERE oauth_authorization_codes.expires_at < $1
)
DELETE FROM oauth_refresh_tokens WHERE oauth_refresh_tokens.expires_at < $1;

-- Grants whose refresh tokens have all been purged can no longer be used.
-- A grant is created in the same transaction as its first refresh token.
-- name: DeleteOrphanOAuthGrants :exec
DELETE FROM oauth_grants g
WHERE NOT EXISTS (SELECT 1 FROM oauth_refresh_tokens t WHERE t.family_id = g.id);
