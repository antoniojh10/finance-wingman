-- The user's workspaces with what deleting the account means for each:
-- how many members and owners it has.
-- name: ListUserWorkspacesForDeletion :many
SELECT w.id, w.name, m.role,
    (SELECT count(*) FROM workspace_members o WHERE o.workspace_id = w.id)::int AS members,
    (SELECT count(*) FROM workspace_members o WHERE o.workspace_id = w.id AND o.role = 'owner')::int AS owners
FROM workspace_members m
JOIN workspaces w ON w.id = m.workspace_id
WHERE m.user_id = $1
ORDER BY lower(w.name), w.created_at;

-- Locks the user's workspaces and their memberships, in a fixed order, so
-- nobody joins, leaves or changes role while the account is deleted
-- (adding a member needs a key share lock on the workspace row).
-- name: LockUserWorkspaces :exec
SELECT 1
FROM workspaces w
JOIN workspace_members m ON m.workspace_id = w.id
WHERE w.id IN (SELECT um.workspace_id FROM workspace_members um WHERE um.user_id = $1)
ORDER BY w.id, m.user_id
FOR UPDATE;

-- name: ScheduleUserDeletion :one
UPDATE users SET deletion_scheduled_for = $2
WHERE id = $1 AND deletion_scheduled_for IS NULL
RETURNING *;

-- name: CancelUserDeletion :execrows
UPDATE users SET deletion_scheduled_for = NULL
WHERE id = $1 AND deletion_scheduled_for IS NOT NULL;

-- Picks one user whose grace period is over and locks them, skipping users
-- another API instance is already deleting.
-- name: ClaimDueUserDeletion :one
SELECT * FROM users
WHERE deletion_scheduled_for <= $1
ORDER BY deletion_scheduled_for
LIMIT 1
FOR UPDATE SKIP LOCKED;

-- Deletes the user with their sessions, login challenges and OAuth grants,
-- codes and tokens (ON DELETE CASCADE). Rows they created in workspaces
-- they leave are kept without attribution (ON DELETE SET NULL).
-- name: DeleteUser :execrows
DELETE FROM users WHERE id = $1;
