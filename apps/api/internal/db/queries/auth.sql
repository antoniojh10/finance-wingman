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
INSERT INTO sessions (user_id, token_hash, client, expires_at)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetSessionByTokenHash :one
SELECT s.*, u.email, u.name, u.locale
FROM sessions s
JOIN users u ON u.id = s.user_id
WHERE s.token_hash = $1;

-- name: TouchSession :exec
UPDATE sessions SET last_used_at = now() WHERE id = $1;

-- name: DeleteSession :exec
DELETE FROM sessions WHERE id = $1;

-- name: DeleteExpiredAuthRecords :exec
WITH expired_sessions AS (
    DELETE FROM sessions WHERE sessions.expires_at < $1
)
DELETE FROM login_challenges WHERE login_challenges.expires_at < $1;
