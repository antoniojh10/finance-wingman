package workspace_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/finance"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/testutil"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/workspace"
)

func addUser(t *testing.T, pool *pgxpool.Pool, email string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := pool.QueryRow(context.Background(), `INSERT INTO users (email) VALUES ($1) RETURNING id`, email).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func role(t *testing.T, pool *pgxpool.Pool, workspaceID, userID uuid.UUID) string {
	t.Helper()
	var r string
	if err := pool.QueryRow(context.Background(), `SELECT role FROM workspace_members WHERE workspace_id = $1 AND user_id = $2`, workspaceID, userID).Scan(&r); err != nil {
		t.Fatalf("membership of %s: %v", userID, err)
	}
	return r
}

func TestCreateMakesTheUserOwner(t *testing.T) {
	t.Parallel()
	pool := testutil.NewDatabase(t, true)
	svc := workspace.NewService(pool)
	ana := addUser(t, pool, "ana@example.com")

	w, err := svc.Create(context.Background(), ana, "  Home  ")
	if err != nil {
		t.Fatal(err)
	}
	if w.Name != "Home" {
		t.Fatalf("expected a trimmed name, got %q", w.Name)
	}
	if r := role(t, pool, w.ID, ana); r != workspace.RoleOwner {
		t.Fatalf("expected owner, got %q", r)
	}

	for _, name := range []string{" ", strings.Repeat("x", 101)} {
		var domainErr *finance.Error
		if _, err := svc.Create(context.Background(), ana, name); !errors.As(err, &domainErr) || domainErr.Kind != finance.KindInvalid {
			t.Fatalf("name %q: expected a validation error, got %v", name, err)
		}
	}
}

func TestBootstrapCreatesTheFirstWorkspaceOnce(t *testing.T) {
	t.Parallel()
	pool := testutil.NewDatabase(t, true)
	svc := workspace.NewService(pool)
	ctx := context.Background()
	ana, bob := addUser(t, pool, "ana@example.com"), addUser(t, pool, "bob@example.com")

	if created, err := svc.Bootstrap(ctx, "Home", nil); err != nil || created {
		t.Fatalf("without initial users nothing is created: created=%v err=%v", created, err)
	}
	created, err := svc.Bootstrap(ctx, "Home", []uuid.UUID{ana, bob})
	if err != nil || !created {
		t.Fatalf("expected the first workspace to be created: created=%v err=%v", created, err)
	}
	var id uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT id FROM workspaces WHERE name = 'Home'`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	if role(t, pool, id, ana) != workspace.RoleOwner || role(t, pool, id, bob) != workspace.RoleOwner {
		t.Fatal("initial users should own the first workspace")
	}

	if created, err := svc.Bootstrap(ctx, "Again", []uuid.UUID{ana}); err != nil || created {
		t.Fatalf("a second bootstrap must be a no-op: created=%v err=%v", created, err)
	}
}
