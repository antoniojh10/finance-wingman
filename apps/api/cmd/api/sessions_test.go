package main

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/auth"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/testutil"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/workspace"
)

func newSessionsFixture(t *testing.T) (*auth.Service, *workspace.Service) {
	t.Helper()
	pool := testutil.NewDatabase(t, true)
	authSvc := auth.NewService(pool, &testutil.MailRecorder{}, auth.Config{WebBaseURL: "http://web.test"}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	workspaces := workspace.NewService(pool, &testutil.MailRecorder{}, workspace.Config{WebBaseURL: "http://web.test"})
	return authSvc, workspaces
}

// runCreate runs `sessions create` and returns the token it printed.
func runCreate(t *testing.T, authSvc *auth.Service, workspaces *workspace.Service, args ...string) string {
	t.Helper()
	var out bytes.Buffer
	if err := sessions(context.Background(), authSvc, workspaces, &out, append([]string{"create"}, args...)); err != nil {
		t.Fatalf("sessions create: %v", err)
	}
	line := out.String()
	token, ok := strings.CutPrefix(line, sessionCookie+"=")
	if !ok || strings.Count(line, "\n") != 1 || !strings.HasSuffix(token, "\n") {
		t.Fatalf("output must be a single %s=TOKEN line, got %q", sessionCookie, line)
	}
	return strings.TrimSuffix(token, "\n")
}

func TestSessionsCreateMintsAUsableSession(t *testing.T) {
	t.Parallel()
	authSvc, workspaces := newSessionsFixture(t)
	ctx := context.Background()

	token := runCreate(t, authSvc, workspaces, "--email", "Ana@Example.com", "--name", "Ana", "--workspace", "Home")
	session, err := authSvc.Authenticate(ctx, token)
	if err != nil {
		t.Fatalf("the token should authenticate: %v", err)
	}
	if session.User.Email != "ana@example.com" || session.User.Name != "Ana" {
		t.Fatalf("unexpected user %+v", session.User)
	}
	if session.Workspace == nil || session.Workspace.Name != "Home" || session.Workspace.Role != workspace.RoleOwner {
		t.Fatalf("the session should act on a workspace owned by the user, got %+v", session.Workspace)
	}
}

func TestSessionsCreateIsIdempotent(t *testing.T) {
	t.Parallel()
	authSvc, workspaces := newSessionsFixture(t)
	ctx := context.Background()

	first := runCreate(t, authSvc, workspaces, "--email", "ana@example.com", "--workspace", "Home")
	second := runCreate(t, authSvc, workspaces, "--email", "ana@example.com", "--workspace", "Home")
	if first == second {
		t.Fatal("every call should mint a new token")
	}
	users, err := authSvc.ListUsers(ctx)
	if err != nil || len(users) != 1 {
		t.Fatalf("the user should be created once, got %+v (%v)", users, err)
	}
	s1, err := authSvc.Authenticate(ctx, first)
	if err != nil {
		t.Fatal(err)
	}
	s2, err := authSvc.Authenticate(ctx, second)
	if err != nil {
		t.Fatal(err)
	}
	if s1.Workspace == nil || s2.Workspace == nil || s1.Workspace.ID != s2.Workspace.ID {
		t.Fatal("the workspace should be reused")
	}
}

func TestSessionsCreateUsesTheExistingWorkspaceByDefault(t *testing.T) {
	t.Parallel()
	authSvc, workspaces := newSessionsFixture(t)
	ctx := context.Background()
	ana, err := authSvc.AddUser(ctx, "ana@example.com", "Ana")
	if err != nil {
		t.Fatal(err)
	}
	home, err := workspaces.Create(ctx, ana.ID, "Home")
	if err != nil {
		t.Fatal(err)
	}

	session, err := authSvc.Authenticate(ctx, runCreate(t, authSvc, workspaces, "--email", "ana@example.com"))
	if err != nil {
		t.Fatal(err)
	}
	if session.Workspace == nil || session.Workspace.ID != home.ID {
		t.Fatalf("expected the existing workspace, got %+v", session.Workspace)
	}
}

func TestSessionsCreateRejectsBadInput(t *testing.T) {
	t.Parallel()
	authSvc, workspaces := newSessionsFixture(t)
	for name, args := range map[string][]string{
		"no subcommand":   {},
		"unknown command": {"delete"},
		"missing email":   {"create"},
		"invalid email":   {"create", "--email", "not-an-email"},
		"unknown flag":    {"create", "--email", "a@example.com", "--nope"},
		"extra argument":  {"create", "--email", "a@example.com", "extra"},
	} {
		t.Run(name, func(t *testing.T) {
			var out bytes.Buffer
			if err := sessions(context.Background(), authSvc, workspaces, &out, args); err == nil {
				t.Fatal("expected an error")
			}
			if out.Len() != 0 {
				t.Fatalf("nothing should be printed on failure, got %q", out.String())
			}
		})
	}
}
