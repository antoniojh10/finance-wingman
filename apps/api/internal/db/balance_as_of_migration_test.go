package db_test

import (
	"context"
	"testing"
	"time"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/db"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/testutil"
)

func TestBalanceAsOfMigrationBackfillsCreationDate(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool := testutil.NewDatabase(t, true)

	m, err := db.NewMigrator(pool)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()

	// Roll the latest migration back (also exercises its Down), create an
	// account in the old schema, then apply it again.
	if err := m.Down(ctx); err != nil {
		t.Fatalf("down: %v", err)
	}
	var id string
	mustScan(t, pool, &id, `INSERT INTO accounts (name, type, currency, initial_balance, created_at)
		VALUES ('Old', 'checking', 'MXN', 50000, '2026-03-15 23:30:00+00') RETURNING id`)
	if _, err := m.Up(ctx); err != nil {
		t.Fatalf("up: %v", err)
	}

	var anchor, createdDate time.Time
	mustScan(t, pool, &anchor, `SELECT balance_as_of FROM accounts WHERE id = $1`, id)
	mustScan(t, pool, &createdDate, `SELECT created_at::date FROM accounts WHERE id = $1`, id)
	if !anchor.Equal(createdDate) {
		t.Fatalf("expected balance_as_of %s (created_at date), got %s", createdDate, anchor)
	}

	// New accounts default to today.
	var isToday bool
	mustScan(t, pool, &isToday, `INSERT INTO accounts (name, type, currency) VALUES ('New', 'cash', 'MXN')
		RETURNING balance_as_of = CURRENT_DATE`)
	if !isToday {
		t.Fatal("expected balance_as_of to default to the current date")
	}
}
