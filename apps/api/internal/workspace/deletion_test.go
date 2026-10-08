package workspace_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/db"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/finance"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/mail"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/testutil"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/workspace"
)

// frozenClock returns a clock fixed at start and a function to move it.
func frozenClock(svc *workspace.Service) func(time.Duration) time.Time {
	now := time.Now().UTC().Truncate(time.Second)
	svc.SetClock(func() time.Time { return now })
	return func(d time.Duration) time.Time {
		now = now.Add(d)
		return now
	}
}

// financeTables are every table holding workspace finance data.
var financeTables = []string{"accounts", "categories", "recurring_items", "transactions", "recurring_dismissed_suggestions", "budgets", "activity_log"}

// seedFinance fills the workspace with one row of every kind of finance
// data, including rows that reference each other.
func seedFinance(t *testing.T, pool *pgxpool.Pool, workspaceID, ownerID uuid.UUID) {
	t.Helper()
	ctx := db.WithWorkspace(context.Background(), workspaceID)
	var checking, savings, category, recurring uuid.UUID
	steps := []struct {
		sql  string
		args []any
		dest *uuid.UUID
	}{
		{`INSERT INTO accounts (name, type, currency, owner_user_id) VALUES ('Checking', 'checking', 'EUR', $1) RETURNING id`, []any{ownerID}, &checking},
		{`INSERT INTO accounts (name, type, currency) VALUES ('Savings', 'savings', 'EUR') RETURNING id`, nil, &savings},
		{`INSERT INTO categories (name, kind) VALUES ('Food', 'expense') RETURNING id`, nil, &category},
	}
	for _, s := range steps {
		if err := pool.QueryRow(ctx, s.sql, s.args...).Scan(s.dest); err != nil {
			t.Fatalf("%s: %v", s.sql, err)
		}
	}
	if err := pool.QueryRow(ctx, `INSERT INTO recurring_items (name, type, account_id, category_id, amount, interval_unit, start_on)
		VALUES ('Gym', 'expense', $1, $2, 3000, 'month', '2026-01-01') RETURNING id`, checking, category).Scan(&recurring); err != nil {
		t.Fatal(err)
	}
	execs := []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO transactions (type, account_id, amount, category_id, occurred_on, recurring_id, recurring_due_on, created_by)
			VALUES ('expense', $1, 3000, $2, '2026-01-01', $3, '2026-01-01', $4)`, []any{checking, category, recurring, ownerID}},
		{`INSERT INTO transactions (type, account_id, amount, destination_account_id, destination_amount, occurred_on)
			VALUES ('transfer', $1, 500, $2, 500, '2026-01-02')`, []any{checking, savings}},
		{`INSERT INTO budgets (category_id, currency, month, amount_minor) VALUES ($1, 'EUR', '2026-01-01', 10000)`, []any{category}},
		{`INSERT INTO recurring_dismissed_suggestions (account_id, type, description) VALUES ($1, 'expense', 'netflix')`, []any{checking}},
		{`INSERT INTO activity_log (actor_id, channel, action, entity_type, entity_id) VALUES ($1, 'web', 'created', 'account', $2)`, []any{ownerID, checking}},
	}
	for _, e := range execs {
		if _, err := pool.Exec(ctx, e.sql, e.args...); err != nil {
			t.Fatalf("%s: %v", e.sql, err)
		}
	}
}

// financeRows counts the workspace's rows in every finance table.
func financeRows(t *testing.T, pool *pgxpool.Pool, workspaceID uuid.UUID) map[string]int {
	t.Helper()
	ctx := db.WithWorkspace(context.Background(), workspaceID)
	counts := map[string]int{}
	for _, table := range financeTables {
		var n int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM `+table+` WHERE workspace_id = $1`, workspaceID).Scan(&n); err != nil {
			t.Fatal(err)
		}
		counts[table] = n
	}
	return counts
}

func count(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
	return n
}

// connectApp records an OAuth client connected to the workspace by the
// user, with its refresh token and an access token session.
func connectApp(t *testing.T, pool *pgxpool.Pool, userID, workspaceID uuid.UUID) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	grant := uuid.New()
	client := "client-" + grant.String()
	for _, q := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO oauth_clients (id, name, redirect_uris, token_endpoint_auth_method) VALUES ($1, 'Claude', '{https://claude.ai/cb}', 'none')`, []any{client}},
		{`INSERT INTO oauth_grants (id, client_id, user_id, workspace_id) VALUES ($1, $2, $3, $4)`, []any{grant, client, userID, workspaceID}},
		{`INSERT INTO oauth_refresh_tokens (token_hash, family_id, client_id, user_id, workspace_id, expires_at)
			VALUES (gen_random_uuid()::text::bytea, $1, $2, $3, $4, now() + interval '30 days')`, []any{grant, client, userID, workspaceID}},
		{`INSERT INTO sessions (user_id, token_hash, client, expires_at, oauth_client_id, oauth_family_id, workspace_id)
			VALUES ($1, gen_random_uuid()::text::bytea, 'Claude', now() + interval '1 hour', $2, $3, $4)`, []any{userID, client, grant, workspaceID}},
	} {
		if _, err := pool.Exec(ctx, q.sql, q.args...); err != nil {
			t.Fatalf("%s: %v", q.sql, err)
		}
	}
	return grant
}

func webSession(t *testing.T, pool *pgxpool.Pool, userID uuid.UUID, workspaceID *uuid.UUID) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := pool.QueryRow(context.Background(), `INSERT INTO sessions (user_id, token_hash, expires_at, workspace_id)
		VALUES ($1, gen_random_uuid()::text::bytea, now() + interval '30 days', $2) RETURNING id`, userID, workspaceID).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func TestScheduleWorkspaceDeletion(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	ctx := context.Background()
	start := frozenClock(f.svc)(0)

	_, err := f.svc.ScheduleDeletion(ctx, f.bob, f.home, "Home")
	expectKind(t, err, finance.KindForbidden)
	_, err = f.svc.ScheduleDeletion(ctx, f.outsider, f.home, "Home")
	expectKind(t, err, finance.KindNotFound)
	for _, name := range []string{"", "home", "Home!"} {
		_, err = f.svc.ScheduleDeletion(ctx, f.ana, f.home, name)
		expectKind(t, err, finance.KindInvalid)
	}
	if n := len(f.mail.Messages()); n != 0 {
		t.Fatalf("refused requests must not email anyone, got %d emails", n)
	}

	w, err := f.svc.ScheduleDeletion(ctx, f.ana, f.home, " Home ")
	if err != nil {
		t.Fatal(err)
	}
	want := start.Add(workspace.DefaultDeletionGracePeriod)
	if w.DeletionScheduledFor == nil || !w.DeletionScheduledFor.Equal(want) {
		t.Fatalf("expected the deletion in 7 days (%v), got %v", want, w.DeletionScheduledFor)
	}
	msgs := f.mail.Messages()
	if len(msgs) != 2 {
		t.Fatalf("every member should be emailed, got %d emails", len(msgs))
	}
	for _, m := range msgs {
		if !strings.Contains(m.Subject, "Home") || !strings.Contains(m.Text, "http://web.test/settings/workspace") || !strings.Contains(m.Text, "backups") {
			t.Fatalf("unexpected email to %s: %s\n%s", m.To, m.Subject, m.Text)
		}
	}

	_, err = f.svc.ScheduleDeletion(ctx, f.ana, f.home, "Home")
	expectKind(t, err, finance.KindConflict)

	list, err := f.svc.List(ctx, f.bob)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].DeletionScheduledFor == nil || !list[0].DeletionScheduledFor.Equal(want) {
		t.Fatalf("members should see the scheduled deletion: %+v", list)
	}
}

func TestCancelWorkspaceDeletion(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	ctx := context.Background()
	advance := frozenClock(f.svc)

	expectKind(t, f.svc.CancelDeletion(ctx, f.ana, f.home), finance.KindNotFound)
	if _, err := f.svc.ScheduleDeletion(ctx, f.ana, f.home, "Home"); err != nil {
		t.Fatal(err)
	}
	expectKind(t, f.svc.CancelDeletion(ctx, f.bob, f.home), finance.KindForbidden)
	expectKind(t, f.svc.CancelDeletion(ctx, f.outsider, f.home), finance.KindNotFound)

	advance(6 * 24 * time.Hour)
	if err := f.svc.CancelDeletion(ctx, f.ana, f.home); err != nil {
		t.Fatalf("cancel within the grace period: %v", err)
	}
	advance(2 * 24 * time.Hour)
	if err := f.svc.ExecuteScheduledDeletions(ctx); err != nil {
		t.Fatal(err)
	}
	if count(t, f.pool, `SELECT count(*) FROM workspaces WHERE id = $1`, f.home) != 1 {
		t.Fatal("a cancelled deletion must not delete the workspace")
	}
	list, _ := f.svc.List(ctx, f.ana)
	if list[0].DeletionScheduledFor != nil {
		t.Fatalf("the workspace should no longer be scheduled: %+v", list)
	}
	// It can be scheduled again.
	if _, err := f.svc.ScheduleDeletion(ctx, f.ana, f.home, "Home"); err != nil {
		t.Fatal(err)
	}
}

func TestExecuteWorkspaceDeletion(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	ctx := context.Background()
	advance := frozenClock(f.svc)

	other, err := f.svc.Create(ctx, f.ana, "Other")
	if err != nil {
		t.Fatal(err)
	}
	seedFinance(t, f.pool, f.home, f.ana)
	seedFinance(t, f.pool, other.ID, f.ana)
	grant := connectApp(t, f.pool, f.bob, f.home)
	otherGrant := connectApp(t, f.pool, f.ana, other.ID)
	browser := webSession(t, f.pool, f.bob, &f.home)
	if _, err := f.svc.Invite(ctx, f.ana, f.home, workspace.InviteInput{Email: "carl@example.com"}); err != nil {
		t.Fatal(err)
	}

	if _, err := f.svc.ScheduleDeletion(ctx, f.ana, f.home, "Home"); err != nil {
		t.Fatal(err)
	}
	sent := len(f.mail.Messages())

	// Still within the grace period: nothing happens.
	advance(workspace.DefaultDeletionGracePeriod - time.Minute)
	if err := f.svc.ExecuteScheduledDeletions(ctx); err != nil {
		t.Fatal(err)
	}
	if count(t, f.pool, `SELECT count(*) FROM workspaces WHERE id = $1`, f.home) != 1 {
		t.Fatal("the workspace was deleted before the grace period ended")
	}

	advance(2 * time.Minute)
	if err := f.svc.ExecuteScheduledDeletions(ctx); err != nil {
		t.Fatal(err)
	}

	if count(t, f.pool, `SELECT count(*) FROM workspaces WHERE id = $1`, f.home) != 0 {
		t.Fatal("the workspace should be deleted")
	}
	for table, n := range financeRows(t, f.pool, f.home) {
		if n != 0 {
			t.Errorf("%s still has %d rows of the deleted workspace", table, n)
		}
	}
	for table, n := range financeRows(t, f.pool, other.ID) {
		if n == 0 {
			t.Errorf("%s lost the rows of another workspace", table)
		}
	}
	if n := count(t, f.pool, `SELECT count(*) FROM workspace_members WHERE workspace_id = $1`, f.home); n != 0 {
		t.Errorf("%d memberships left", n)
	}
	if n := count(t, f.pool, `SELECT count(*) FROM workspace_invitations WHERE workspace_id = $1`, f.home); n != 0 {
		t.Errorf("%d invitations left", n)
	}
	if n := count(t, f.pool, `SELECT count(*) FROM oauth_grants WHERE id = $1`, grant); n != 0 {
		t.Error("the app connected to the workspace should be disconnected")
	}
	if n := count(t, f.pool, `SELECT count(*) FROM sessions WHERE oauth_family_id = $1`, grant); n != 0 {
		t.Error("the access tokens of the app should be revoked")
	}
	if n := count(t, f.pool, `SELECT count(*) FROM oauth_grants WHERE id = $1`, otherGrant); n != 1 {
		t.Error("apps connected to other workspaces must stay connected")
	}
	if n := count(t, f.pool, `SELECT count(*) FROM sessions WHERE id = $1 AND workspace_id IS NULL`, browser); n != 1 {
		t.Error("browser sessions should stay signed in without a workspace")
	}
	if n := count(t, f.pool, `SELECT count(*) FROM users WHERE id IN ($1, $2)`, f.ana, f.bob); n != 2 {
		t.Error("members' users must be kept")
	}

	deleted := f.mail.Messages()[sent:]
	if len(deleted) != 2 {
		t.Fatalf("every member should be told about the deletion, got %d emails", len(deleted))
	}
	for _, m := range deleted {
		if !strings.Contains(m.Subject, "was deleted") || !strings.Contains(m.Text, "Home") {
			t.Fatalf("unexpected email: %s\n%s", m.Subject, m.Text)
		}
	}

	// Running again does nothing more.
	if err := f.svc.ExecuteScheduledDeletions(ctx); err != nil {
		t.Fatal(err)
	}
	if len(f.mail.Messages()) != sent+2 {
		t.Fatal("a deletion must be carried out once")
	}
}

func TestWorkspaceDeletionEmailsUseEachMembersLanguage(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	ctx := context.Background()
	if _, err := f.pool.Exec(ctx, `UPDATE users SET locale = 'es', name = 'Bob' WHERE id = $1`, f.bob); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.ScheduleDeletion(ctx, f.ana, f.home, "Home"); err != nil {
		t.Fatal(err)
	}
	for _, m := range f.mail.Messages() {
		spanish := strings.Contains(m.Subject, "se eliminará")
		if (m.To == "bob@example.com") != spanish {
			t.Fatalf("email to %s in the wrong language: %s", m.To, m.Subject)
		}
	}
}

func TestDeletionNeedsAnEmailToBeSent(t *testing.T) {
	t.Parallel()
	pool := testutil.NewDatabase(t, true)
	svc := workspace.NewService(pool, failingSender{}, workspace.Config{WebBaseURL: "http://web.test"})
	ana := addUser(t, pool, "ana@example.com")
	w, err := svc.Create(context.Background(), ana, "Home")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ScheduleDeletion(context.Background(), ana, w.ID, "Home"); err == nil {
		t.Fatal("expected the failed email to be reported")
	}
	if count(t, pool, `SELECT count(*) FROM workspaces WHERE id = $1 AND deletion_scheduled_for IS NULL`, w.ID) != 1 {
		t.Fatal("a deletion whose email failed must not be scheduled")
	}
}

type failingSender struct{}

func (failingSender) Send(context.Context, mail.Message) error { return errors.New("mail server down") }
