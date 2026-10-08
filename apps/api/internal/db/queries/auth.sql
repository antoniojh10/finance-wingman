-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = $1;

-- name: GetUser :one
SELECT * FROM users WHERE id = $1;

-- name: ListUsers :many
SELECT * FROM users ORDER BY created_at;

-- name: UpsertUser :one
INSERT INTO users (email, name)
VALUES ($1, $2)
ON CONFLICT (email) DO UPDATE SET name = CASE WHEN EXCLUDED.name = '' THEN users.name ELSE EXCLUDED.name END
RETURNING *;

-- name: UpdateUser :one
UPDATE users SET
    name = COALESCE(sqlc.narg('name'), name),
    locale = COALESCE(sqlc.narg('locale'), locale)
WHERE id = sqlc.arg('id')
RETURNING *;

-- name: DeleteUserByEmail :execrows
DELETE FROM users WHERE email = $1;

-- name: CountRecentChallenges :one
SELECT count(*) FROM login_challenges
WHERE user_id = $1 AND created_at > $2;

-- name: CreateChallenge :one
INSERT INTO login_challenges (user_id, token_hash, code_hash, expires_at)
VALUES ($1, $2, $3, $4)
RETURNING id;

-- name: GetChallengeByTokenHash :one
SELECT * FROM login_challenges WHERE token_hash = $1;

-- name: GetLatestOpenChallenge :one
SELECT * FROM login_challenges
WHERE user_id = $1 AND consumed_at IS NULL AND expires_at > $2
ORDER BY created_at DESC
LIMIT 1;

-- name: IncrementChallengeAttempts :one
UPDATE login_challenges SET attempts = attempts + 1 WHERE id = $1 RETURNING attempts;

-- Consumes the challenge only if it is still open, so concurrent redemptions
-- cannot both succeed.
-- name: ConsumeChallenge :execrows
UPDATE login_challenges SET consumed_at = now()
WHERE id = $1 AND consumed_at IS NULL;

-- Invalidates every other open challenge once the user has logged in.
-- name: ConsumeOpenChallenges :exec
UPDATE login_challenges SET consumed_at = now()
WHERE user_id = $1 AND consumed_at IS NULL;

-- name: CreateSession :one
INSERT INTO sessions (user_id, token_hash, client, expires_at, workspace_id, user_agent, last_used_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- The session's workspace is only reported while the user is still a
-- member of it.
-- name: GetSessionByTokenHash :one
SELECT s.*, u.email, u.name, u.locale,
    w.id AS active_workspace_id, w.name AS workspace_name, m.role AS workspace_role
FROM sessions s
JOIN users u ON u.id = s.user_id
LEFT JOIN workspace_members m ON m.workspace_id = s.workspace_id AND m.user_id = s.user_id
LEFT JOIN workspaces w ON w.id = m.workspace_id
WHERE s.token_hash = $1;

-- The workspace a new session starts in: the one the user used last, or
-- the first one they joined.
-- name: GetDefaultWorkspaceID :one
SELECT m.workspace_id
FROM workspace_members m
WHERE m.user_id = $1
ORDER BY (
    SELECT max(s.last_used_at) FROM sessions s
    WHERE s.user_id = m.user_id AND s.workspace_id = m.workspace_id
) DESC NULLS LAST, m.created_at
LIMIT 1;

-- name: GetMembership :one
SELECT w.id, w.name, m.role
FROM workspace_members m
JOIN workspaces w ON w.id = m.workspace_id
WHERE m.workspace_id = $1 AND m.user_id = $2;

-- Switching counts as using the session, so the next sign-in resumes the
-- workspace the user switched to.
-- name: SetSessionWorkspace :exec
UPDATE sessions SET workspace_id = $2, last_used_at = now() WHERE id = $1;

-- name: TouchSession :exec
UPDATE sessions SET last_used_at = $2 WHERE id = $1;

-- name: DeleteSession :exec
DELETE FROM sessions WHERE id = $1;

-- The user's active sign-ins (not the access tokens of connected apps),
-- most recently used first.
-- name: ListUserSessions :many
SELECT id, user_agent, created_at, last_used_at, expires_at
FROM sessions
WHERE user_id = sqlc.arg('user_id')
  AND oauth_family_id IS NULL
  AND expires_at > sqlc.arg('now')
  AND last_used_at > sqlc.arg('idle_since')
ORDER BY last_used_at DESC, created_at DESC;

-- Only the user's own sign-ins can be revoked this way; connected apps are
-- disconnected through their OAuth grant.
-- name: DeleteUserSession :execrows
DELETE FROM sessions
WHERE id = $1 AND user_id = $2 AND oauth_family_id IS NULL;

-- name: DeleteOtherUserSessions :execrows
DELETE FROM sessions
WHERE user_id = $1 AND id <> $2 AND oauth_family_id IS NULL;

-- Sessions end at their absolute expiry or after being idle too long.
-- name: DeleteExpiredAuthRecords :exec
WITH expired_sessions AS (
    DELETE FROM sessions
    WHERE sessions.expires_at < sqlc.arg('now') OR sessions.last_used_at < sqlc.arg('idle_since')
)
DELETE FROM login_challenges WHERE login_challenges.expires_at < sqlc.arg('now');
