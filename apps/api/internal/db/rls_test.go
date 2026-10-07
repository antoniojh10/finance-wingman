package db_test

import (
	"context"
	"testing"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/db"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/testutil"
)

const insufficientPrivilege = "42501"

func TestAPIRunsAsAppRole(t *testing.T) {
	t.Parallel()
	pool := testutil.NewDatabase(t, true)

	var user string
	var super, bypass bool
	mustScan(t, context.Background(), pool, &user, `SELECT current_user`)
	mustScan(t, context.Background(), pool, &super, `SELECT rolsuper FROM pg_roles WHERE rolname = current_user`)
	mustScan(t, context.Background(), pool, &bypass, `SELECT rolbypassrls FROM pg_roles WHERE rolname = current_user`)
	if user != db.AppRole || super || bypass {
		t.Fatalf("expected %s without superuser or BYPASSRLS, got %s (super=%v bypassrls=%v)", db.AppRole, user, super, bypass)
	}

	_, err := pool.Exec(context.Background(), `INSERT INTO currencies (code, name, symbol, minor_units) VALUES ('XAU', 'Gold', 'Au', 2)`)
	if got := pgCode(err); got != insufficientPrivilege {
		t.Fatalf("currencies should be read-only, got %q (err: %v)", got, err)
	}
	_, err = pool.Exec(context.Background(), `SELECT * FROM goose_db_version`)
	if got := pgCode(err); got != insufficientPrivilege {
		t.Fatalf("goose_db_version should not be readable, got %q (err: %v)", got, err)
	}
}

func TestRowLevelSecurityIsolatesWorkspaces(t *testing.T) {
	t.Parallel()
	pool := testutil.NewDatabase(t, true)
	home := db.WithWorkspace(context.Background(), testutil.NewWorkspace(t, pool, "Home"))
	otherID := testutil.NewWorkspace(t, pool, "Other")
	other := db.WithWorkspace(context.Background(), otherID)

	var account string
	mustScan(t, home, pool, &account, `INSERT INTO accounts (name, type, currency) VALUES ('Checking', 'checking', 'MXN') RETURNING id`)
	if _, err := pool.Exec(home, `INSERT INTO transactions (type, account_id, amount, occurred_on) VALUES ('expense', $1, 500, CURRENT_DATE)`, account); err != nil {
		t.Fatal(err)
	}

	count := func(ctx context.Context, table string) int {
		t.Helper()
		var n int
		mustScan(t, ctx, pool, &n, `SELECT count(*) FROM `+table)
		return n
	}
	if n := count(home, "transactions"); n != 1 {
		t.Fatalf("home should see its transaction, got %d", n)
	}
	for _, table := range []string{"accounts", "transactions"} {
		if n := count(other, table); n != 0 {
			t.Fatalf("other workspace sees %d rows of %s", n, table)
		}
		if n := count(context.Background(), table); n != 0 {
			t.Fatalf("no workspace sees %d rows of %s", n, table)
		}
	}

	// Writes from another workspace can't reach the row.
	tag, err := pool.Exec(other, `UPDATE accounts SET name = 'Hijacked' WHERE id = $1`, account)
	if err != nil || tag.RowsAffected() != 0 {
		t.Fatalf("update from other workspace: rows=%d err=%v", tag.RowsAffected(), err)
	}
	tag, err = pool.Exec(other, `DELETE FROM transactions`)
	if err != nil || tag.RowsAffected() != 0 {
		t.Fatalf("delete from other workspace: rows=%d err=%v", tag.RowsAffected(), err)
	}

	// Rows can't be written into another workspace, nor without one.
	_, err = pool.Exec(home, `INSERT INTO accounts (workspace_id, name, type, currency) VALUES ($1, 'Planted', 'cash', 'MXN')`, otherID)
	if got := pgCode(err); got != insufficientPrivilege {
		t.Fatalf("insert into another workspace: expected %s, got %q (err: %v)", insufficientPrivilege, got, err)
	}
	_, err = pool.Exec(home, `UPDATE accounts SET workspace_id = $1 WHERE id = $2`, otherID, account)
	if got := pgCode(err); got != insufficientPrivilege {
		t.Fatalf("move row to another workspace: expected %s, got %q (err: %v)", insufficientPrivilege, got, err)
	}
	if _, err = pool.Exec(context.Background(), `INSERT INTO categories (name, kind) VALUES ('Food', 'expense')`); err == nil {
		t.Fatal("insert without a workspace should fail")
	}

	// References must stay inside the workspace, whatever ids are guessed.
	_, err = pool.Exec(other, `INSERT INTO transactions (type, account_id, amount, occurred_on) VALUES ('expense', $1, 500, CURRENT_DATE)`, account)
	if got := pgCode(err); got != foreignKeyViolation {
		t.Fatalf("reference to another workspace's account: expected %s, got %q (err: %v)", foreignKeyViolation, got, err)
	}

	// Names are unique per workspace only.
	if _, err := pool.Exec(other, `INSERT INTO accounts (name, type, currency) VALUES ('Checking', 'checking', 'MXN')`); err != nil {
		t.Fatalf("same account name in another workspace should be allowed: %v", err)
	}
}

func TestPooledConnectionsForgetTheWorkspace(t *testing.T) {
	t.Parallel()
	pool := testutil.NewDatabase(t, true)
	home := db.WithWorkspace(context.Background(), testutil.NewWorkspace(t, pool, "Home"))

	var setting string
	mustScan(t, home, pool, &setting, `SELECT current_setting('app.workspace_id')`)
	if setting == "" {
		t.Fatal("expected the workspace to be set")
	}
	// Every acquire overwrites the setting, so whichever connection the pool
	// hands out next must not act on the previous workspace.
	for range 5 {
		mustScan(t, context.Background(), pool, &setting, `SELECT current_setting('app.workspace_id')`)
		if setting != "" {
			t.Fatalf("connection kept workspace %q", setting)
		}
	}
}
