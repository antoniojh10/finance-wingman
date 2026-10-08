package workspace_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/db"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/finance"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/workspace"
)

func outcomes(d workspace.AccountDeletion) map[string]string {
	out := map[string]string{}
	for _, w := range d.Workspaces {
		out[w.Name] = w.Outcome
	}
	return out
}

func TestAccountDeletionOutcomes(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	ctx := context.Background()
	if _, err := f.svc.Create(ctx, f.ana, "Solo"); err != nil {
		t.Fatal(err)
	}
	shared, err := f.svc.Create(ctx, f.outsider, "Shared")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.pool.Exec(ctx, `INSERT INTO workspace_members (workspace_id, user_id, role) VALUES ($1, $2, 'owner')`, shared.ID, f.ana); err != nil {
		t.Fatal(err)
	}

	d, err := f.svc.AccountDeletion(ctx, f.ana)
	if err != nil {
		t.Fatal(err)
	}
	got := outcomes(d)
	want := map[string]string{"Home": workspace.OutcomeBlocked, "Shared": workspace.OutcomeLeave, "Solo": workspace.OutcomeDelete}
	if d.ScheduledFor != nil || len(got) != 3 || got["Home"] != want["Home"] || got["Shared"] != want["Shared"] || got["Solo"] != want["Solo"] {
		t.Fatalf("unexpected outcomes for ana: %+v", d)
	}
	d, err = f.svc.AccountDeletion(ctx, f.bob)
	if err != nil {
		t.Fatal(err)
	}
	if got := outcomes(d); len(got) != 1 || got["Home"] != workspace.OutcomeLeave {
		t.Fatalf("a member just leaves: %+v", d)
	}
}

func TestScheduleAccountDeletion(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	ctx := context.Background()
	start := frozenClock(f.svc)(0)
	if _, err := f.svc.Create(ctx, f.ana, "Solo"); err != nil {
		t.Fatal(err)
	}

	_, err := f.svc.ScheduleAccountDeletion(ctx, f.ana, "bob@example.com")
	expectKind(t, err, finance.KindInvalid)
	// Ana is the only owner of Home, which bob also belongs to.
	_, err = f.svc.ScheduleAccountDeletion(ctx, f.ana, "ana@example.com")
	expectKind(t, err, finance.KindConflict)
	if !strings.Contains(err.Error(), `"Home"`) {
		t.Fatalf("the error should name the blocking workspace: %v", err)
	}
	if len(f.mail.Messages()) != 0 {
		t.Fatal("a refused deletion must not email")
	}

	if err := f.svc.SetRole(ctx, f.ana, f.home, f.bob, workspace.RoleOwner); err != nil {
		t.Fatal(err)
	}
	d, err := f.svc.ScheduleAccountDeletion(ctx, f.ana, " ANA@example.com ")
	if err != nil {
		t.Fatal(err)
	}
	want := start.Add(workspace.DefaultDeletionGracePeriod)
	if d.ScheduledFor == nil || !d.ScheduledFor.Equal(want) {
		t.Fatalf("expected the deletion in 7 days (%v): %+v", want, d)
	}
	msgs := f.mail.Messages()
	if len(msgs) != 1 || msgs[0].To != "ana@example.com" {
		t.Fatalf("expected one email to ana: %+v", msgs)
	}
	for _, s := range []string{"“Solo”", "http://web.test/settings/security", "backups"} {
		if !strings.Contains(msgs[0].Text, s) {
			t.Fatalf("email without %q:\n%s", s, msgs[0].Text)
		}
	}

	_, err = f.svc.ScheduleAccountDeletion(ctx, f.ana, "ana@example.com")
	expectKind(t, err, finance.KindConflict)
	got, err := f.svc.AccountDeletion(ctx, f.ana)
	if err != nil || got.ScheduledFor == nil || !got.ScheduledFor.Equal(want) {
		t.Fatalf("the deletion should be reported as scheduled: %+v %v", got, err)
	}
}

func TestCancelAccountDeletion(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	ctx := context.Background()
	advance := frozenClock(f.svc)

	expectKind(t, f.svc.CancelAccountDeletion(ctx, f.bob), finance.KindNotFound)
	if _, err := f.svc.ScheduleAccountDeletion(ctx, f.bob, "bob@example.com"); err != nil {
		t.Fatal(err)
	}
	advance(6 * 24 * time.Hour)
	if err := f.svc.CancelAccountDeletion(ctx, f.bob); err != nil {
		t.Fatalf("cancel within the grace period: %v", err)
	}
	advance(2 * 24 * time.Hour)
	if err := f.svc.ExecuteScheduledDeletions(ctx); err != nil {
		t.Fatal(err)
	}
	if count(t, f.pool, `SELECT count(*) FROM users WHERE id = $1 AND deletion_scheduled_for IS NULL`, f.bob) != 1 {
		t.Fatal("a cancelled deletion must keep the user")
	}
	if role(t, f.pool, f.home, f.bob) != workspace.RoleMember {
		t.Fatal("a cancelled deletion must keep the memberships")
	}
}

func TestExecuteAccountDeletion(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	ctx := context.Background()
	advance := frozenClock(f.svc)

	solo, err := f.svc.Create(ctx, f.ana, "Solo")
	if err != nil {
		t.Fatal(err)
	}
	if err := f.svc.SetRole(ctx, f.ana, f.home, f.bob, workspace.RoleOwner); err != nil {
		t.Fatal(err)
	}
	seedFinance(t, f.pool, solo.ID, f.ana)
	seedFinance(t, f.pool, f.home, f.ana) // ana owns "Checking"; "Savings" is shared
	homeCtx := db.WithWorkspace(ctx, f.home)
	// A shared account with the name of ana's, which becomes shared too.
	if _, err := f.pool.Exec(homeCtx, `INSERT INTO accounts (name, type, currency) VALUES ('Checking', 'checking', 'EUR')`); err != nil {
		t.Fatal(err)
	}
	grant := connectApp(t, f.pool, f.ana, f.home)
	browser := webSession(t, f.pool, f.ana, &f.home)
	bobSession := webSession(t, f.pool, f.bob, &f.home)
	if _, err := f.pool.Exec(ctx, `INSERT INTO login_challenges (user_id, token_hash, code_hash, expires_at) VALUES ($1, 'x', 'y', now() + interval '1 hour')`, f.ana); err != nil {
		t.Fatal(err)
	}

	if _, err := f.svc.ScheduleAccountDeletion(ctx, f.ana, "ana@example.com"); err != nil {
		t.Fatal(err)
	}
	sent := len(f.mail.Messages())

	advance(workspace.DefaultDeletionGracePeriod - time.Minute)
	if err := f.svc.ExecuteScheduledDeletions(ctx); err != nil {
		t.Fatal(err)
	}
	if count(t, f.pool, `SELECT count(*) FROM users WHERE id = $1`, f.ana) != 1 {
		t.Fatal("the account was deleted before the grace period ended")
	}

	advance(2 * time.Minute)
	if err := f.svc.ExecuteScheduledDeletions(ctx); err != nil {
		t.Fatal(err)
	}

	checks := []struct {
		what string
		sql  string
		args []any
		want int
	}{
		{"the user", `SELECT count(*) FROM users WHERE id = $1`, []any{f.ana}, 0},
		{"their memberships", `SELECT count(*) FROM workspace_members WHERE user_id = $1`, []any{f.ana}, 0},
		{"their browser session", `SELECT count(*) FROM sessions WHERE id = $1`, []any{browser}, 0},
		{"their app sessions", `SELECT count(*) FROM sessions WHERE oauth_family_id = $1`, []any{grant}, 0},
		{"their OAuth grant", `SELECT count(*) FROM oauth_grants WHERE id = $1`, []any{grant}, 0},
		{"their refresh tokens", `SELECT count(*) FROM oauth_refresh_tokens WHERE family_id = $1`, []any{grant}, 0},
		{"their login challenges", `SELECT count(*) FROM login_challenges WHERE user_id = $1`, []any{f.ana}, 0},
		{"the workspace where they were alone", `SELECT count(*) FROM workspaces WHERE id = $1`, []any{solo.ID}, 0},
		{"the workspace they shared", `SELECT count(*) FROM workspaces WHERE id = $1`, []any{f.home}, 1},
		{"the other member", `SELECT count(*) FROM workspace_members WHERE workspace_id = $1 AND user_id = $2 AND role = 'owner'`, []any{f.home, f.bob}, 1},
		{"the other member's session", `SELECT count(*) FROM sessions WHERE id = $1 AND workspace_id = $2`, []any{bobSession, f.home}, 1},
	}
	for _, c := range checks {
		if n := count(t, f.pool, c.sql, c.args...); n != c.want {
			t.Errorf("%s: %d rows, want %d", c.what, n, c.want)
		}
	}
	for table, n := range financeRows(t, f.pool, solo.ID) {
		if n != 0 {
			t.Errorf("%s still has %d rows of the deleted workspace", table, n)
		}
	}
	for table, n := range financeRows(t, f.pool, f.home) {
		if n == 0 {
			t.Errorf("%s lost the rows of the shared workspace", table)
		}
	}

	// Ana's account in Home is now shared, renamed so it doesn't clash.
	rows, err := f.pool.Query(homeCtx, `SELECT name, owner_user_id FROM accounts ORDER BY name`)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for rows.Next() {
		var name string
		var owner *uuid.UUID
		if err := rows.Scan(&name, &owner); err != nil {
			t.Fatal(err)
		}
		if owner != nil {
			t.Errorf("account %q should be shared", name)
		}
		names = append(names, name)
	}
	if strings.Join(names, ",") != "Checking,Checking (ana@example.com),Savings" {
		t.Fatalf("unexpected accounts: %v", names)
	}
	var unattributed int
	if err := f.pool.QueryRow(homeCtx, `SELECT count(*) FROM transactions WHERE created_by IS NULL`).Scan(&unattributed); err != nil || unattributed != 2 {
		t.Errorf("transactions created by ana should be kept without attribution: %d %v", unattributed, err)
	}

	deleted := f.mail.Messages()[sent:]
	if len(deleted) != 1 || deleted[0].To != "ana@example.com" || !strings.Contains(deleted[0].Subject, "was deleted") || !strings.Contains(deleted[0].Text, "“Solo”") {
		t.Fatalf("expected an account deleted email naming Solo: %+v", deleted)
	}

	if err := f.svc.ExecuteScheduledDeletions(ctx); err != nil {
		t.Fatal(err)
	}
	if len(f.mail.Messages()) != sent+1 {
		t.Fatal("a deletion must be carried out once")
	}
}

// Ownership can change during the grace period; the deletion is then
// cancelled rather than leaving a workspace without an owner.
func TestAccountDeletionBlockedAtExecution(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	ctx := context.Background()
	advance := frozenClock(f.svc)
	solo, err := f.svc.Create(ctx, f.ana, "Solo")
	if err != nil {
		t.Fatal(err)
	}
	if err := f.svc.SetRole(ctx, f.ana, f.home, f.bob, workspace.RoleOwner); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.ScheduleAccountDeletion(ctx, f.ana, "ana@example.com"); err != nil {
		t.Fatal(err)
	}
	// Bob steps down, leaving ana as Home's only owner.
	if err := f.svc.SetRole(ctx, f.bob, f.home, f.bob, workspace.RoleMember); err != nil {
		t.Fatal(err)
	}
	sent := len(f.mail.Messages())

	advance(workspace.DefaultDeletionGracePeriod + time.Minute)
	if err := f.svc.ExecuteScheduledDeletions(ctx); err != nil {
		t.Fatal(err)
	}
	if count(t, f.pool, `SELECT count(*) FROM users WHERE id = $1 AND deletion_scheduled_for IS NULL`, f.ana) != 1 {
		t.Fatal("the user should be kept and their deletion cancelled")
	}
	if count(t, f.pool, `SELECT count(*) FROM workspaces WHERE id = $1`, solo.ID) != 1 || role(t, f.pool, f.home, f.ana) != workspace.RoleOwner {
		t.Fatal("nothing should be deleted when the deletion is blocked")
	}
	msgs := f.mail.Messages()[sent:]
	if len(msgs) != 1 || !strings.Contains(msgs[0].Subject, "couldn't delete") || !strings.Contains(msgs[0].Text, "“Home”") {
		t.Fatalf("expected an email explaining the cancellation: %+v", msgs)
	}
}

func TestAccountDeletionEmailsInSpanish(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	ctx := context.Background()
	advance := frozenClock(f.svc)
	if _, err := f.pool.Exec(ctx, `UPDATE users SET locale = 'es' WHERE id = $1`, f.bob); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.ScheduleAccountDeletion(ctx, f.bob, "bob@example.com"); err != nil {
		t.Fatal(err)
	}
	advance(workspace.DefaultDeletionGracePeriod + time.Minute)
	if err := f.svc.ExecuteScheduledDeletions(ctx); err != nil {
		t.Fatal(err)
	}
	msgs := f.mail.Messages()
	if len(msgs) != 2 || !strings.Contains(msgs[0].Subject, "se eliminará") || !strings.Contains(msgs[1].Subject, "fue eliminada") {
		t.Fatalf("expected both emails in Spanish: %+v", msgs)
	}
}
