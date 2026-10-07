package auth_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/auth"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/testutil"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/workspace"
)

func newWorkspaceFixture(t *testing.T) (*auth.Service, *pgxpool.Pool, auth.User) {
	t.Helper()
	pool := testutil.NewDatabase(t, true)
	svc := auth.NewService(pool, &testutil.MailRecorder{}, auth.Config{WebBaseURL: "http://web.test"}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	user, err := svc.AddUser(context.Background(), "ana@example.com", "Ana")
	if err != nil {
		t.Fatal(err)
	}
	return svc, pool, user
}

func TestSessionsStartInTheLastUsedWorkspace(t *testing.T) {
	t.Parallel()
	svc, pool, ana := newWorkspaceFixture(t)
	ctx := context.Background()
	workspaces := workspace.NewService(pool, &testutil.MailRecorder{}, workspace.Config{WebBaseURL: "http://web.test"})

	session, err := svc.CreateSession(ctx, ana.ID, auth.ClientWeb, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if session.Workspace != nil {
		t.Fatalf("no workspaces yet, got %+v", session.Workspace)
	}

	home, err := workspaces.Create(ctx, ana.ID, "Home")
	if err != nil {
		t.Fatal(err)
	}
	trip, err := workspaces.Create(ctx, ana.ID, "Trip")
	if err != nil {
		t.Fatal(err)
	}
	session, err = svc.CreateSession(ctx, ana.ID, auth.ClientWeb, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if session.Workspace == nil || session.Workspace.ID != home.ID || session.Workspace.Role != workspace.RoleOwner {
		t.Fatalf("a first session should use the oldest membership, got %+v", session.Workspace)
	}

	switched, err := svc.SwitchWorkspace(ctx, session, trip.ID)
	if err != nil {
		t.Fatal(err)
	}
	if switched.Workspace.ID != trip.ID || switched.Workspace.Name != "Trip" {
		t.Fatalf("unexpected workspace after switching: %+v", switched.Workspace)
	}
	next, err := svc.CreateSession(ctx, ana.ID, auth.ClientWeb, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if next.Workspace == nil || next.Workspace.ID != trip.ID {
		t.Fatalf("a new session should resume the last used workspace, got %+v", next.Workspace)
	}
}

func TestSwitchWorkspaceRequiresMembership(t *testing.T) {
	t.Parallel()
	svc, pool, ana := newWorkspaceFixture(t)
	ctx := context.Background()
	bob, err := svc.AddUser(ctx, "bob@example.com", "Bob")
	if err != nil {
		t.Fatal(err)
	}
	bobs, err := workspace.NewService(pool, &testutil.MailRecorder{}, workspace.Config{WebBaseURL: "http://web.test"}).Create(ctx, bob.ID, "Bob's")
	if err != nil {
		t.Fatal(err)
	}
	session, err := svc.CreateSession(ctx, ana.ID, auth.ClientWeb, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SwitchWorkspace(ctx, session, bobs.ID); !errors.Is(err, auth.ErrNotMember) {
		t.Fatalf("expected ErrNotMember, got %v", err)
	}
	if _, err := svc.CreateWorkspaceSession(ctx, ana.ID, &bobs.ID, auth.ClientWeb, time.Hour); !errors.Is(err, auth.ErrNotMember) {
		t.Fatalf("expected ErrNotMember, got %v", err)
	}
}

func TestSessionLosesWorkspaceWhenMembershipEnds(t *testing.T) {
	t.Parallel()
	svc, pool, ana := newWorkspaceFixture(t)
	ctx := context.Background()
	home, err := workspace.NewService(pool, &testutil.MailRecorder{}, workspace.Config{WebBaseURL: "http://web.test"}).Create(ctx, ana.ID, "Home")
	if err != nil {
		t.Fatal(err)
	}
	session, err := svc.CreateSession(ctx, ana.ID, auth.ClientWeb, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	got, err := svc.Authenticate(ctx, session.Token)
	if err != nil || got.Workspace == nil || got.Workspace.ID != home.ID {
		t.Fatalf("expected the session to act on Home: %+v %v", got.Workspace, err)
	}

	if _, err := pool.Exec(ctx, `DELETE FROM workspace_members WHERE user_id = $1`, ana.ID); err != nil {
		t.Fatal(err)
	}
	got, err = svc.Authenticate(ctx, session.Token)
	if err != nil {
		t.Fatal(err)
	}
	if got.Workspace != nil {
		t.Fatalf("expected no workspace after leaving, got %+v", got.Workspace)
	}
}
