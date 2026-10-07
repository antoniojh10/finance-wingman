package workspace_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/finance"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/testutil"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/workspace"
)

type fixture struct {
	pool     *pgxpool.Pool
	svc      *workspace.Service
	mail     *testutil.MailRecorder
	ana, bob uuid.UUID // ana owns home, bob is a member
	home     uuid.UUID
	outsider uuid.UUID
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	pool := testutil.NewDatabase(t, true)
	mail := &testutil.MailRecorder{}
	f := fixture{pool: pool, mail: mail, svc: workspace.NewService(pool, mail, workspace.Config{WebBaseURL: "http://web.test"})}
	f.ana, f.bob, f.outsider = addUser(t, pool, "ana@example.com"), addUser(t, pool, "bob@example.com"), addUser(t, pool, "eve@example.com")
	w, err := f.svc.Create(context.Background(), f.ana, "Home")
	if err != nil {
		t.Fatal(err)
	}
	f.home = w.ID
	if _, err := pool.Exec(context.Background(), `INSERT INTO workspace_members (workspace_id, user_id, role) VALUES ($1, $2, 'member')`, f.home, f.bob); err != nil {
		t.Fatal(err)
	}
	return f
}

func expectKind(t *testing.T, err error, kind finance.ErrorKind) {
	t.Helper()
	var domainErr *finance.Error
	if !errors.As(err, &domainErr) || domainErr.Kind != kind {
		t.Fatalf("expected error kind %d, got %v", kind, err)
	}
}

func TestListAndRename(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	ctx := context.Background()
	if _, err := f.svc.Create(ctx, f.bob, "Bob's"); err != nil {
		t.Fatal(err)
	}

	list, err := f.svc.List(ctx, f.bob)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || list[0].Name != "Bob's" || list[0].Role != workspace.RoleOwner || list[1].Name != "Home" || list[1].Role != workspace.RoleMember {
		t.Fatalf("unexpected workspaces: %+v", list)
	}

	if w, err := f.svc.Rename(ctx, f.ana, f.home, "Casa"); err != nil || w.Name != "Casa" {
		t.Fatalf("owner rename: %+v %v", w, err)
	}
	_, err = f.svc.Rename(ctx, f.bob, f.home, "Mine")
	expectKind(t, err, finance.KindForbidden)
	_, err = f.svc.Rename(ctx, f.outsider, f.home, "Mine")
	expectKind(t, err, finance.KindNotFound)
}

func TestMembers(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	ctx := context.Background()

	members, err := f.svc.Members(ctx, f.bob, f.home)
	if err != nil {
		t.Fatal(err)
	}
	if len(members) != 2 || members[0].UserID != f.ana || members[0].Role != workspace.RoleOwner || members[1].Email != "bob@example.com" {
		t.Fatalf("unexpected members: %+v", members)
	}
	_, err = f.svc.Members(ctx, f.outsider, f.home)
	expectKind(t, err, finance.KindNotFound)
}

func TestSetRole(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	ctx := context.Background()

	expectKind(t, f.svc.SetRole(ctx, f.bob, f.home, f.bob, workspace.RoleOwner), finance.KindForbidden)
	expectKind(t, f.svc.SetRole(ctx, f.ana, f.home, f.ana, workspace.RoleMember), finance.KindConflict)
	expectKind(t, f.svc.SetRole(ctx, f.ana, f.home, f.outsider, workspace.RoleOwner), finance.KindNotFound)
	expectKind(t, f.svc.SetRole(ctx, f.ana, f.home, f.bob, "admin"), finance.KindInvalid)

	if err := f.svc.SetRole(ctx, f.ana, f.home, f.bob, workspace.RoleOwner); err != nil {
		t.Fatal(err)
	}
	// With a second owner, the first one can step down.
	if err := f.svc.SetRole(ctx, f.ana, f.home, f.ana, workspace.RoleMember); err != nil {
		t.Fatal(err)
	}
	if role(t, f.pool, f.home, f.ana) != workspace.RoleMember || role(t, f.pool, f.home, f.bob) != workspace.RoleOwner {
		t.Fatal("roles were not swapped")
	}
}

func TestRemoveMember(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	ctx := context.Background()
	carl := addUser(t, f.pool, "carl@example.com")
	if _, err := f.pool.Exec(ctx, `INSERT INTO workspace_members (workspace_id, user_id, role) VALUES ($1, $2, 'member')`, f.home, carl); err != nil {
		t.Fatal(err)
	}
	var session uuid.UUID
	if err := f.pool.QueryRow(ctx, `INSERT INTO sessions (user_id, token_hash, expires_at, workspace_id) VALUES ($1, 'x', now() + interval '1 hour', $2) RETURNING id`, f.bob, f.home).Scan(&session); err != nil {
		t.Fatal(err)
	}

	// Members can't remove others, only themselves.
	expectKind(t, f.svc.RemoveMember(ctx, f.bob, f.home, carl), finance.KindForbidden)
	if err := f.svc.RemoveMember(ctx, f.bob, f.home, f.bob); err != nil {
		t.Fatalf("leave: %v", err)
	}
	var workspaceID *uuid.UUID
	if err := f.pool.QueryRow(ctx, `SELECT workspace_id FROM sessions WHERE id = $1`, session).Scan(&workspaceID); err != nil {
		t.Fatal(err)
	}
	if workspaceID != nil {
		t.Fatal("sessions should stop acting on a workspace the user left")
	}

	if err := f.svc.RemoveMember(ctx, f.ana, f.home, carl); err != nil {
		t.Fatalf("owner removes member: %v", err)
	}
	expectKind(t, f.svc.RemoveMember(ctx, f.ana, f.home, f.ana), finance.KindConflict)
	expectKind(t, f.svc.RemoveMember(ctx, f.ana, f.home, carl), finance.KindNotFound)
	expectKind(t, f.svc.RemoveMember(ctx, f.outsider, f.home, f.ana), finance.KindNotFound)
}
