-- name: CreateWorkspace :one
INSERT INTO workspaces (name) VALUES ($1) RETURNING *;

-- name: CountWorkspaces :one
SELECT count(*) FROM workspaces;

-- name: AddWorkspaceMember :exec
INSERT INTO workspace_members (workspace_id, user_id, role)
VALUES ($1, $2, $3)
ON CONFLICT (workspace_id, user_id) DO NOTHING;

-- name: ListUserWorkspaces :many
SELECT w.id, w.name, w.created_at, m.role
FROM workspace_members m
JOIN workspaces w ON w.id = m.workspace_id
WHERE m.user_id = $1
ORDER BY lower(w.name), w.created_at;

-- name: GetMemberRole :one
SELECT role FROM workspace_members WHERE workspace_id = $1 AND user_id = $2;

-- name: GetWorkspace :one
SELECT * FROM workspaces WHERE id = $1;

-- name: RenameWorkspace :one
UPDATE workspaces SET name = $2 WHERE id = $1 RETURNING *;

-- name: ListWorkspaceMembers :many
SELECT u.id, u.email, u.name, m.role, m.created_at
FROM workspace_members m
JOIN users u ON u.id = m.user_id
WHERE m.workspace_id = $1
ORDER BY m.created_at, u.email;

-- Locks the workspace's memberships so concurrent role changes can't leave
-- it without an owner.
-- name: LockWorkspaceMembers :many
SELECT user_id, role FROM workspace_members WHERE workspace_id = $1 FOR UPDATE;

-- name: UpdateMemberRole :execrows
UPDATE workspace_members SET role = $3 WHERE workspace_id = $1 AND user_id = $2;

-- name: RemoveWorkspaceMember :execrows
DELETE FROM workspace_members WHERE workspace_id = $1 AND user_id = $2;

-- Sessions acting on a workspace the user left stop acting on it.
-- name: ClearSessionsWorkspace :exec
UPDATE sessions SET workspace_id = NULL WHERE workspace_id = $1 AND user_id = $2;

-- name: IsMemberByEmail :one
SELECT EXISTS (
    SELECT 1 FROM workspace_members m JOIN users u ON u.id = m.user_id
    WHERE m.workspace_id = $1 AND u.email = $2
);

-- name: CountRecentInvitations :one
SELECT count(*) FROM workspace_invitations WHERE workspace_id = $1 AND created_at > $2;

-- Revokes the open invitation for the address, so a new one replaces it.
-- name: RevokeOpenInvitationsForEmail :exec
UPDATE workspace_invitations SET revoked_at = now()
WHERE workspace_id = $1 AND email = $2 AND accepted_at IS NULL AND revoked_at IS NULL;

-- name: CreateInvitation :one
INSERT INTO workspace_invitations (workspace_id, email, role, token_hash, invited_by, expires_at)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ListOpenInvitations :many
SELECT i.id, i.email, i.role, i.expires_at, i.created_at, u.name AS invited_by_name, u.email AS invited_by_email
FROM workspace_invitations i
LEFT JOIN users u ON u.id = i.invited_by
WHERE i.workspace_id = $1 AND i.accepted_at IS NULL AND i.revoked_at IS NULL
ORDER BY i.created_at DESC;

-- name: RevokeInvitation :execrows
UPDATE workspace_invitations SET revoked_at = now()
WHERE id = $1 AND workspace_id = $2 AND accepted_at IS NULL AND revoked_at IS NULL;

-- name: GetInvitationByTokenHash :one
SELECT i.*, w.name AS workspace_name, u.name AS invited_by_name, u.email AS invited_by_email
FROM workspace_invitations i
JOIN workspaces w ON w.id = i.workspace_id
LEFT JOIN users u ON u.id = i.invited_by
WHERE i.token_hash = $1;

-- Marks the invitation accepted only if it was still open.
-- name: AcceptInvitation :one
UPDATE workspace_invitations SET accepted_at = now()
WHERE token_hash = $1 AND accepted_at IS NULL AND revoked_at IS NULL AND expires_at > $2
RETURNING *;

-- name: DeleteExpiredInvitations :exec
DELETE FROM workspace_invitations WHERE expires_at < $1 AND accepted_at IS NULL;
