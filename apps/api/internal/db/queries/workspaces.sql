-- name: CreateWorkspace :one
INSERT INTO workspaces (name) VALUES ($1) RETURNING *;

-- name: CountWorkspaces :one
SELECT count(*) FROM workspaces;

-- name: AddWorkspaceMember :exec
INSERT INTO workspace_members (workspace_id, user_id, role)
VALUES ($1, $2, $3)
ON CONFLICT (workspace_id, user_id) DO NOTHING;
