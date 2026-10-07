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
