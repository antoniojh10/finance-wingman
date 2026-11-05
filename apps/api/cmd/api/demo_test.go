package main

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/auth"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/demo"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/finance"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/groceries"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/testutil"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/workspace"
)

func TestDemoSeedCommand(t *testing.T) {
	t.Parallel()
	pool := testutil.NewDatabase(t, true)
	mail := &testutil.MailRecorder{}
	authSvc := auth.NewService(pool, mail, auth.Config{WebBaseURL: "http://web.test"}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	fin := finance.NewService(pool, nil)
	seeder := &demo.Seeder{
		Auth:       authSvc,
		Workspaces: workspace.NewService(pool, mail, workspace.Config{WebBaseURL: "http://web.test"}),
		Finance:    fin,
		Groceries:  groceries.NewService(pool, fin),
	}
	ctx := context.Background()
	if _, err := authSvc.AddUser(ctx, "ana@example.com", "Ana"); err != nil {
		t.Fatal(err)
	}

	run := func(args ...string) (string, error) {
		var out bytes.Buffer
		err := demoCommand(ctx, seeder, &out, args)
		return out.String(), err
	}

	for _, args := range [][]string{nil, {"seed"}, {"unknown"}, {"seed", "--email", "ana@example.com", "extra"}, {"seed", "--nope"}} {
		if _, err := run(args...); err == nil || !strings.Contains(err.Error(), "usage:") {
			t.Errorf("%v: expected a usage error, got %v", args, err)
		}
	}
	if _, err := run("seed", "--email", "nobody@example.com"); err == nil {
		t.Error("an unknown user should be an error")
	}

	out, err := run("seed", "--email", "ana@example.com", "--workspace", "Showcase")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `seeded workspace "Showcase"`) || !strings.Contains(out, "6 accounts") || !strings.Contains(out, "10 receipts") {
		t.Fatalf("unexpected summary: %q", out)
	}
	if _, err := run("seed", "--email", "ana@example.com", "--workspace", "Showcase"); err == nil || !strings.Contains(err.Error(), "--reset") {
		t.Fatalf("expected a hint to use --reset, got %v", err)
	}
	if _, err := run("seed", "--email", "ana@example.com", "--workspace", "Showcase", "--reset", "--locale", "es"); err != nil {
		t.Fatal(err)
	}
}
