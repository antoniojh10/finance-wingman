package db_test

import (
	"context"
	"testing"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/db"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/testutil"
)

// Existing groups whose name is exactly a default one get its key; renamed
// and custom groups stay without. Rolling back drops the column.
func TestProductGroupDefaultKeyMigration(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool := testutil.NewDatabase(t, false)
	m, err := db.NewMigrator(pool)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	const before, after = 20261015120623, 20261016191006
	if err := m.UpTo(ctx, before); err != nil {
		t.Fatal(err)
	}

	ws := testutil.NewWorkspace(t, pool, "Home")
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, "SELECT set_config('app.workspace_id', $1, true)", ws.String()); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO product_groups (name, is_food, position) VALUES
		('Meat & fish', true, 0), ('Other', false, 1), ('Meat and fish', true, 2), ('Pets', false, 3)`); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	if err := m.UpTo(ctx, after); err != nil {
		t.Fatal(err)
	}
	var keys string
	mustScan(t, ctx, pool, &keys, `SELECT string_agg(coalesce(default_key, '-'), ',' ORDER BY position) FROM product_groups`)
	if keys != "meat_fish,other,-,-" {
		t.Fatalf("unexpected keys %q", keys)
	}

	if err := m.DownTo(ctx, before); err != nil {
		t.Fatal(err)
	}
	var columns int
	mustScan(t, ctx, pool, &columns, `SELECT count(*) FROM information_schema.columns WHERE table_name = 'product_groups' AND column_name = 'default_key'`)
	if columns != 0 {
		t.Fatal("default_key should be dropped on rollback")
	}
	if _, err := m.Up(ctx); err != nil {
		t.Fatal(err)
	}
}
