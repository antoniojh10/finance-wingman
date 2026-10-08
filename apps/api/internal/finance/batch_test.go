package finance_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/db"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/finance"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/testutil"
)

func newBatchFixture(t *testing.T) (context.Context, *finance.Service, finance.Account) {
	t.Helper()
	pool := testutil.NewDatabase(t, true)
	svc := finance.NewService(pool, time.UTC)
	ctx := db.WithWorkspace(context.Background(), testutil.NewWorkspace(t, pool, "Home"))
	account, err := svc.CreateAccount(ctx, finance.CreateAccountInput{Name: "Card", Type: "checking", Currency: "MXN", BalanceAsOf: "2000-01-01"})
	if err != nil {
		t.Fatal(err)
	}
	return ctx, svc, account
}

func TestUpdateTransactions(t *testing.T) {
	t.Parallel()
	ctx, svc, account := newBatchFixture(t)
	created, err := svc.CreateTransactions(ctx, []finance.TransactionInput{
		{Type: finance.TypeExpense, AccountID: account.ID, Amount: 1000, OccurredOn: "2026-05-01"},
		{Type: finance.TypeExpense, AccountID: account.ID, Amount: 2000, OccurredOn: "2026-05-02"},
	})
	if err != nil {
		t.Fatal(err)
	}

	updated, err := svc.UpdateTransactions(ctx, []finance.TransactionUpdate{
		{ID: created[0].ID, Input: finance.TransactionInput{Type: finance.TypeExpense, AccountID: account.ID, Amount: 1500, Description: "a", OccurredOn: "2026-05-01"}},
		{ID: created[1].ID, Input: finance.TransactionInput{Type: finance.TypeExpense, AccountID: account.ID, Amount: 2000, Description: "b", OccurredOn: "2026-05-03"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(updated) != 2 || updated[0].Amount != 1500 || updated[0].Description != "a" || updated[1].OccurredOn != "2026-05-03" {
		t.Fatalf("unexpected result: %+v", updated)
	}
}

func TestUpdateTransactionsIsAtomic(t *testing.T) {
	t.Parallel()
	ctx, svc, account := newBatchFixture(t)
	created, err := svc.CreateTransactions(ctx, []finance.TransactionInput{
		{Type: finance.TypeExpense, AccountID: account.ID, Amount: 1000, OccurredOn: "2026-05-01"},
	})
	if err != nil {
		t.Fatal(err)
	}
	valid := finance.TransactionInput{Type: finance.TypeExpense, AccountID: account.ID, Amount: 9999, OccurredOn: "2026-05-01"}

	cases := map[string][]finance.TransactionUpdate{
		"unknown id": {
			{ID: created[0].ID, Input: valid},
			{ID: uuid.New(), Input: valid},
		},
		"invalid input": {
			{ID: created[0].ID, Input: valid},
			{ID: created[0].ID, Input: finance.TransactionInput{Type: finance.TypeExpense, AccountID: account.ID, Amount: 0}},
		},
	}
	for name, items := range cases {
		_, err := svc.UpdateTransactions(ctx, items)
		if err == nil || !strings.Contains(err.Error(), "items[1]") {
			t.Fatalf("%s: expected an error naming items[1], got %v", name, err)
		}
		got, err := svc.GetTransaction(ctx, created[0].ID)
		if err != nil || got.Amount != 1000 {
			t.Fatalf("%s: the first item must be rolled back: %+v %v", name, got, err)
		}
	}
}

func TestUpdateTransactionsValidatesBatch(t *testing.T) {
	t.Parallel()
	ctx, svc, account := newBatchFixture(t)
	in := finance.TransactionInput{Type: finance.TypeExpense, AccountID: account.ID, Amount: 1}
	id := uuid.New()

	if _, err := svc.UpdateTransactions(ctx, nil); err == nil || !strings.Contains(err.Error(), "at least one") {
		t.Fatalf("empty batch should fail: %v", err)
	}
	if _, err := svc.UpdateTransactions(ctx, []finance.TransactionUpdate{{ID: id, Input: in}, {ID: id, Input: in}}); err == nil || !strings.Contains(err.Error(), "items[1].id") {
		t.Fatalf("duplicate ids should fail: %v", err)
	}
	items := make([]finance.TransactionUpdate, finance.MaxBatchSize+1)
	for i := range items {
		items[i] = finance.TransactionUpdate{ID: uuid.New(), Input: in}
	}
	if _, err := svc.UpdateTransactions(ctx, items); err == nil || !strings.Contains(err.Error(), "at most") {
		t.Fatalf("oversized batch should fail: %v", err)
	}
}
