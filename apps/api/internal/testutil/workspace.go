package testutil

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// NewWorkspace creates an empty workspace and returns its id. Use
// db.WithWorkspace to act on it.
func NewWorkspace(t *testing.T, pool *pgxpool.Pool, name string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := pool.QueryRow(context.Background(), `INSERT INTO workspaces (name) VALUES ($1) RETURNING id`, name).Scan(&id); err != nil {
		t.Fatalf("create workspace %q: %v", name, err)
	}
	return id
}

// NewMember creates a user and adds them to the workspace as a member,
// returning the user id.
func NewMember(t *testing.T, pool *pgxpool.Pool, workspaceID uuid.UUID, email, name string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	var id uuid.UUID
	if err := pool.QueryRow(ctx, `INSERT INTO users (email, name) VALUES ($1, $2) RETURNING id`, email, name).Scan(&id); err != nil {
		t.Fatalf("create user %q: %v", email, err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO workspace_members (workspace_id, user_id, role) VALUES ($1, $2, 'member')`, workspaceID, id); err != nil {
		t.Fatalf("add member %q: %v", email, err)
	}
	return id
}
