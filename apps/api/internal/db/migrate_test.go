package db_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/db"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/testutil"
)

func TestMigrationsRoundTrip(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool := testutil.NewDatabase(t, false)

	m, err := db.NewMigrator(pool)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()

	applied, err := m.Up(ctx)
	if err != nil {
		t.Fatalf("up: %v", err)
	}
	if applied == 0 {
		t.Fatal("expected at least one migration to be applied")
	}

	if again, err := m.Up(ctx); err != nil || again != 0 {
		t.Fatalf("second up should be a no-op, got applied=%d err=%v", again, err)
	}

	if err := m.Reset(ctx); err != nil {
		t.Fatalf("reset: %v", err)
	}
	var tables int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM information_schema.tables WHERE table_schema = 'public' AND table_name <> 'goose_db_version'`,
	).Scan(&tables); err != nil {
		t.Fatal(err)
	}
	if tables != 0 {
		t.Fatalf("expected no tables after reset, found %d", tables)
	}

	if _, err := m.Up(ctx); err != nil {
		t.Fatalf("up after reset: %v", err)
	}

	status, err := m.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range status {
		if s.State != "applied" {
			t.Fatalf("migration %d not applied: %s", s.Source.Version, s.State)
		}
	}
}

// Authorizations made before OAuth grants existed become grants, so
// connected apps keep refreshing their tokens after the upgrade.
func TestOAuthGrantsBackfill(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool := testutil.NewDatabase(t, false)
	m, err := db.NewMigrator(pool)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	if err := m.UpTo(ctx, 20261008083443); err != nil {
		t.Fatal(err)
	}

	var userID, workspaceID string
	mustScan(t, ctx, pool, &userID, `INSERT INTO users (email) VALUES ('ana@example.com') RETURNING id`)
	mustScan(t, ctx, pool, &workspaceID, `INSERT INTO workspaces (name) VALUES ('Home') RETURNING id`)
	if _, err := pool.Exec(ctx, `INSERT INTO oauth_clients (id, name, redirect_uris, token_endpoint_auth_method)
		VALUES ('fw_claude', 'Claude', '{https://claude.ai/cb}', 'none')`); err != nil {
		t.Fatal(err)
	}
	const active, revoked = "11111111-1111-1111-1111-111111111111", "22222222-2222-2222-2222-222222222222"
	if _, err := pool.Exec(ctx, `INSERT INTO oauth_refresh_tokens (token_hash, family_id, client_id, user_id, scope, workspace_id, expires_at, revoked_at, created_at) VALUES
		('a1', $1, 'fw_claude', $3, 'finance', $4, now() + interval '80 days', now() - interval '10 days', now() - interval '20 days'),
		('a2', $1, 'fw_claude', $3, 'finance', $4, now() + interval '89 days', NULL, now() - interval '10 days'),
		('r1', $2, 'fw_claude', $3, 'finance', NULL, now() + interval '50 days', now() - interval '1 day', now() - interval '40 days')`,
		active, revoked, userID, workspaceID); err != nil {
		t.Fatal(err)
	}

	if _, err := m.Up(ctx); err != nil {
		t.Fatal(err)
	}
	var (
		count                      int
		activeOK, revokedOK, ageOK bool
	)
	mustScan(t, ctx, pool, &count, `SELECT count(*) FROM oauth_grants`)
	if count != 2 {
		t.Fatalf("expected 2 grants, got %d", count)
	}
	mustScan(t, ctx, pool, &activeOK, `SELECT revoked_at IS NULL AND client_id = 'fw_claude' AND user_id = $2 AND workspace_id = $3 AND scope = 'finance'
		FROM oauth_grants WHERE id = $1`, active, userID, workspaceID)
	mustScan(t, ctx, pool, &ageOK, `SELECT created_at < now() - interval '19 days' AND last_used_at > now() - interval '11 days'
		FROM oauth_grants WHERE id = $1`, active)
	mustScan(t, ctx, pool, &revokedOK, `SELECT revoked_at IS NOT NULL AND workspace_id IS NULL FROM oauth_grants WHERE id = $1`, revoked)
	if !activeOK || !ageOK || !revokedOK {
		t.Fatalf("unexpected backfill: active=%v age=%v revoked=%v", activeOK, ageOK, revokedOK)
	}
}

// schemaFixture holds ids of rows created for constraint tests.
type schemaFixture struct {
	ctx      context.Context // acts on the fixture's workspace
	pool     *pgxpool.Pool
	mxn, usd string // account ids
	expense  string // category id
}

func newSchemaFixture(t *testing.T) schemaFixture {
	t.Helper()
	f := schemaFixture{pool: testutil.NewDatabase(t, true)}
	f.ctx = db.WithWorkspace(context.Background(), testutil.NewWorkspace(t, f.pool, "Home"))
	mustScan(t, f.ctx, f.pool, &f.mxn, `INSERT INTO accounts (name, type, currency) VALUES ('Checking', 'checking', 'MXN') RETURNING id`)
	mustScan(t, f.ctx, f.pool, &f.usd, `INSERT INTO accounts (name, type, currency) VALUES ('Savings USD', 'savings', 'USD') RETURNING id`)
	mustScan(t, f.ctx, f.pool, &f.expense, `INSERT INTO categories (name, kind) VALUES ('Food', 'expense') RETURNING id`)
	return f
}

func mustScan(t *testing.T, ctx context.Context, pool *pgxpool.Pool, dest any, sql string, args ...any) {
	t.Helper()
	if err := pool.QueryRow(ctx, sql, args...).Scan(dest); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
}

func pgCode(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code
	}
	return ""
}

const (
	checkViolation      = "23514"
	foreignKeyViolation = "23503"
	uniqueViolation     = "23505"
)

func TestSchemaConstraints(t *testing.T) {
	t.Parallel()
	f := newSchemaFixture(t)
	ctx := f.ctx

	insertTx := `INSERT INTO transactions (type, account_id, amount, destination_account_id, destination_amount, category_id, occurred_on)
		VALUES ($1, $2, $3, $4, $5, $6, CURRENT_DATE)`

	valid := []struct {
		name string
		args []any
	}{
		{"expense with category", []any{"expense", f.mxn, 15000, nil, nil, f.expense}},
		{"income without category", []any{"income", f.mxn, 100000, nil, nil, nil}},
		{"cross-currency transfer", []any{"transfer", f.mxn, 170000, f.usd, 10000, nil}},
	}
	for _, tc := range valid {
		t.Run("accepts "+tc.name, func(t *testing.T) {
			if _, err := f.pool.Exec(ctx, insertTx, tc.args...); err != nil {
				t.Fatalf("expected insert to succeed: %v", err)
			}
		})
	}

	invalid := []struct {
		name string
		code string
		args []any
	}{
		{"zero amount", checkViolation, []any{"expense", f.mxn, 0, nil, nil, nil}},
		{"negative amount", checkViolation, []any{"income", f.mxn, -5, nil, nil, nil}},
		{"unknown type", checkViolation, []any{"refund", f.mxn, 100, nil, nil, nil}},
		{"transfer without destination", checkViolation, []any{"transfer", f.mxn, 100, nil, nil, nil}},
		{"transfer without destination amount", checkViolation, []any{"transfer", f.mxn, 100, f.usd, nil, nil}},
		{"transfer to same account", checkViolation, []any{"transfer", f.mxn, 100, f.mxn, 100, nil}},
		{"transfer with category", checkViolation, []any{"transfer", f.mxn, 100, f.usd, 5, f.expense}},
		{"expense with destination", checkViolation, []any{"expense", f.mxn, 100, f.usd, nil, nil}},
		{"unknown account", foreignKeyViolation, []any{"expense", "00000000-0000-0000-0000-000000000000", 100, nil, nil, nil}},
	}
	for _, tc := range invalid {
		t.Run("rejects "+tc.name, func(t *testing.T) {
			_, err := f.pool.Exec(ctx, insertTx, tc.args...)
			if got := pgCode(err); got != tc.code {
				t.Fatalf("expected SQLSTATE %s, got %q (err: %v)", tc.code, got, err)
			}
		})
	}
}

func TestAccountConstraints(t *testing.T) {
	t.Parallel()
	f := newSchemaFixture(t)
	ctx := f.ctx

	_, err := f.pool.Exec(ctx, `INSERT INTO accounts (name, type, currency) VALUES ('Wallet', 'cash', 'XXX')`)
	if got := pgCode(err); got != foreignKeyViolation {
		t.Fatalf("unknown currency: expected %s, got %q", foreignKeyViolation, got)
	}

	_, err = f.pool.Exec(ctx, `INSERT INTO accounts (name, type, currency) VALUES ('Wallet', 'piggy_bank', 'MXN')`)
	if got := pgCode(err); got != checkViolation {
		t.Fatalf("unknown type: expected %s, got %q", checkViolation, got)
	}

	_, err = f.pool.Exec(ctx, `INSERT INTO accounts (name, type, currency) VALUES ('   ', 'cash', 'MXN')`)
	if got := pgCode(err); got != checkViolation {
		t.Fatalf("blank name: expected %s, got %q", checkViolation, got)
	}

	_, err = f.pool.Exec(ctx, `INSERT INTO accounts (name, type, currency) VALUES ('CHECKING', 'checking', 'USD')`)
	if got := pgCode(err); got != uniqueViolation {
		t.Fatalf("duplicate active name: expected %s, got %q", uniqueViolation, got)
	}

	// Archiving frees the name for reuse.
	if _, err := f.pool.Exec(ctx, `UPDATE accounts SET archived_at = now() WHERE id = $1`, f.mxn); err != nil {
		t.Fatal(err)
	}
	if _, err := f.pool.Exec(ctx, `INSERT INTO accounts (name, type, currency) VALUES ('Checking', 'checking', 'MXN')`); err != nil {
		t.Fatalf("name of archived account should be reusable: %v", err)
	}
}

func TestAccountBalanceAsOfDefaultsToToday(t *testing.T) {
	t.Parallel()
	f := newSchemaFixture(t)

	var isToday bool
	mustScan(t, f.ctx, f.pool, &isToday, `SELECT balance_as_of = CURRENT_DATE FROM accounts WHERE id = $1`, f.mxn)
	if !isToday {
		t.Fatal("expected balance_as_of to default to the current date")
	}
}

func TestCategoryConstraints(t *testing.T) {
	t.Parallel()
	f := newSchemaFixture(t)
	ctx := f.ctx

	_, err := f.pool.Exec(ctx, `INSERT INTO categories (name, kind) VALUES ('food', 'expense')`)
	if got := pgCode(err); got != uniqueViolation {
		t.Fatalf("duplicate name/kind: expected %s, got %q", uniqueViolation, got)
	}
	if _, err := f.pool.Exec(ctx, `INSERT INTO categories (name, kind) VALUES ('Food', 'income')`); err != nil {
		t.Fatalf("same name with another kind should be allowed: %v", err)
	}
	_, err = f.pool.Exec(ctx, `INSERT INTO categories (name, kind, color) VALUES ('Rent', 'expense', 'red')`)
	if got := pgCode(err); got != checkViolation {
		t.Fatalf("invalid color: expected %s, got %q", checkViolation, got)
	}

	// Deleting a category keeps its transactions, uncategorized.
	var txID string
	mustScan(t, f.ctx, f.pool, &txID, `INSERT INTO transactions (type, account_id, amount, category_id, occurred_on)
		VALUES ('expense', $1, 500, $2, CURRENT_DATE) RETURNING id`, f.mxn, f.expense)
	if _, err := f.pool.Exec(ctx, `DELETE FROM categories WHERE id = $1`, f.expense); err != nil {
		t.Fatal(err)
	}
	var categoryID *string
	mustScan(t, f.ctx, f.pool, &categoryID, `SELECT category_id FROM transactions WHERE id = $1`, txID)
	if categoryID != nil {
		t.Fatalf("expected category to be cleared, got %v", *categoryID)
	}
}

func TestUpdatedAtTrigger(t *testing.T) {
	t.Parallel()
	f := newSchemaFixture(t)

	var before, after time.Time
	mustScan(t, f.ctx, f.pool, &before, `SELECT updated_at FROM accounts WHERE id = $1`, f.mxn)
	// now() is fixed per transaction, so separate statements are enough.
	mustScan(t, f.ctx, f.pool, &after, `UPDATE accounts SET name = 'Main checking' WHERE id = $1 RETURNING updated_at`, f.mxn)
	if !after.After(before) {
		t.Fatalf("updated_at was not bumped: before=%s after=%s", before, after)
	}
}

func TestUserEmailIsCaseInsensitive(t *testing.T) {
	t.Parallel()
	f := newSchemaFixture(t)
	ctx := f.ctx

	if _, err := f.pool.Exec(ctx, `INSERT INTO users (email) VALUES ('Ana@Example.com')`); err != nil {
		t.Fatal(err)
	}
	_, err := f.pool.Exec(ctx, `INSERT INTO users (email) VALUES ('ana@example.com')`)
	if got := pgCode(err); got != uniqueViolation {
		t.Fatalf("expected %s for duplicate email, got %q", uniqueViolation, got)
	}
}

func TestAccountOwners(t *testing.T) {
	t.Parallel()
	pool := testutil.NewDatabase(t, true)
	home := testutil.NewWorkspace(t, pool, "Home")
	other := testutil.NewWorkspace(t, pool, "Other")
	ctx := db.WithWorkspace(context.Background(), home)
	ana := testutil.NewMember(t, pool, home, "ana@example.com", "Ana")
	luis := testutil.NewMember(t, pool, home, "luis@example.com", "Luis")
	outsider := testutil.NewMember(t, pool, other, "eve@example.com", "Eve")

	insert := `INSERT INTO accounts (name, type, currency, owner_user_id) VALUES ($1, 'checking', 'EUR', $2) RETURNING id`
	var anaBNP, luisBNP, shared string
	mustScan(t, ctx, pool, &anaBNP, insert, "BNP", ana)
	mustScan(t, ctx, pool, &luisBNP, insert, "bnp", luis)
	mustScan(t, ctx, pool, &shared, insert, "BNP", nil)

	for _, tc := range []struct {
		name  string
		owner any
		code  string
	}{
		{"same name for the same owner", ana, uniqueViolation},
		{"same name for shared accounts", nil, uniqueViolation},
		{"owner outside the workspace", outsider, foreignKeyViolation},
	} {
		_, err := pool.Exec(ctx, insert, "BNP", tc.owner)
		if got := pgCode(err); got != tc.code {
			t.Fatalf("%s: expected SQLSTATE %s, got %q (err: %v)", tc.name, tc.code, got, err)
		}
	}

	// A member leaving the workspace turns their accounts into shared ones,
	// even outside a workspace context (memberships are not isolated). The
	// shared BNP is archived first so the names don't clash.
	if _, err := pool.Exec(ctx, `UPDATE accounts SET archived_at = now() WHERE id = $1`, shared); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(context.Background(), `DELETE FROM workspace_members WHERE workspace_id = $1 AND user_id = $2`, home, luis); err != nil {
		t.Fatal(err)
	}
	var owner *string
	mustScan(t, ctx, pool, &owner, `SELECT owner_user_id::text FROM accounts WHERE id = $1`, luisBNP)
	if owner != nil {
		t.Fatalf("expected the account to become shared, owner is %s", *owner)
	}
	mustScan(t, ctx, pool, &owner, `SELECT owner_user_id::text FROM accounts WHERE id = $1`, anaBNP)
	if owner == nil || *owner != ana.String() {
		t.Fatalf("other members keep their accounts, got %v", owner)
	}
}
