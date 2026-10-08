package mcpserver

import (
	"fmt"
	"strings"
	"testing"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/finance"
)

func (h *harness) addExpense(args map[string]any) transactionOut {
	h.t.Helper()
	var out transactionOut
	h.mustCall("add_expense", args, &out)
	return out
}

func TestUpdateTransactionPartialKeepsOtherFields(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.account("Card", "MXN")
	h.category("Food", "expense")
	h.category("Fun", "expense")
	tx := h.addExpense(map[string]any{"amount": 10, "account": "Card", "category": "Food", "description": "Lunch", "date": "2026-05-01"})

	var out transactionOut
	h.mustCall("update_transaction", map[string]any{"id": tx.ID, "category": "fun"}, &out)
	if out.ID != tx.ID || out.Category != "Fun" || out.Amount != "10.00" || out.Description != "Lunch" || out.Date != "2026-05-01" || out.Account != "Card" {
		t.Fatalf("only the category should change: %+v", out)
	}

	h.mustCall("update_transaction", map[string]any{"id": tx.ID, "amount": 12.5, "date": "2026-05-02", "description": "Dinner"}, &out)
	if out.Category != "Fun" || out.Amount != "12.50" || out.Date != "2026-05-02" || out.Description != "Dinner" {
		t.Fatalf("unexpected update: %+v", out)
	}
}

func TestUpdateTransactionClearCategory(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.account("Card", "MXN")
	h.category("Food", "expense")
	tx := h.addExpense(map[string]any{"amount": 10, "account": "Card", "category": "Food"})

	var out transactionOut
	h.mustCall("update_transaction", map[string]any{"id": tx.ID, "clear_category": true}, &out)
	if out.Category != "" {
		t.Fatalf("category should be cleared: %+v", out)
	}
	h.mustFail("update_transaction", map[string]any{"id": tx.ID, "category": "Food", "clear_category": true}, "cannot be combined")
}

func TestUpdateTransactionMovesAccount(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.account("Card", "MXN")
	h.account("Cash", "MXN")
	tx := h.addExpense(map[string]any{"amount": 10, "account": "Card"})

	var out transactionOut
	h.mustCall("update_transaction", map[string]any{"id": tx.ID, "account": "Cash"}, &out)
	if out.Account != "Cash" || out.Amount != "10.00" {
		t.Fatalf("unexpected account move: %+v", out)
	}
}

func TestUpdateTransactionKeepsRecurringLink(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	account := h.account("Card", "MXN")
	h.category("Subscriptions", "expense")
	item, err := h.svc.CreateRecurringItem(h.ctx, finance.CreateRecurringItemInput{
		Name: "Netflix", Type: "expense", AccountID: account.ID, Amount: 18900, IntervalUnit: "month",
	})
	if err != nil {
		t.Fatal(err)
	}
	paid, err := h.svc.RegisterRecurringPayment(h.ctx, item.ID, finance.RecurringPaymentInput{})
	if err != nil {
		t.Fatal(err)
	}

	var out transactionOut
	h.mustCall("update_transaction", map[string]any{"id": paid.ID.String(), "category": "Subscriptions", "amount": 200}, &out)
	if out.RecurringID != item.ID.String() || out.Category != "Subscriptions" || out.Amount != "200.00" {
		t.Fatalf("the recurring link must survive an edit: %+v", out)
	}
}

func TestUpdateTransactionTransfer(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.account("Card", "MXN")
	h.account("Cash", "MXN")
	h.account("Dollars", "USD")
	var tr transactionOut
	h.mustCall("add_transfer", map[string]any{"amount": 100, "from_account": "Card", "to_account": "Cash"}, &tr)

	var out transactionOut
	h.mustCall("update_transaction", map[string]any{"id": tr.ID, "amount": 150, "description": "Top up"}, &out)
	if out.Amount != "150.00" || out.DestinationAmount != "150.00" || out.ToAccount != "Cash" || out.Description != "Top up" {
		t.Fatalf("the destination should follow the amount in the same currency: %+v", out)
	}

	h.mustFail("update_transaction", map[string]any{"id": tr.ID, "to_account": "Dollars"}, "destination_amount")
	h.mustCall("update_transaction", map[string]any{"id": tr.ID, "to_account": "Dollars", "destination_amount": 8.25}, &out)
	if out.ToAccount != "Dollars" || out.DestinationAmount != "8.25" || out.DestinationCurrency != "USD" || out.Amount != "150.00" {
		t.Fatalf("unexpected transfer update: %+v", out)
	}
	h.mustCall("update_transaction", map[string]any{"id": tr.ID, "destination_amount": 9}, &out)
	if out.DestinationAmount != "9.00" {
		t.Fatalf("unexpected destination amount: %+v", out)
	}
}

func TestUpdateTransactionErrors(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.account("Card", "MXN")
	h.account("Cash", "MXN")
	h.category("Food", "expense")
	h.category("Salary", "income")
	exp := h.addExpense(map[string]any{"amount": 10, "account": "Card", "category": "Food"})
	var tr transactionOut
	h.mustCall("add_transfer", map[string]any{"amount": 5, "from_account": "Card", "to_account": "Cash"}, &tr)

	h.mustFail("update_transaction", map[string]any{"id": "not-a-uuid", "amount": 1}, "id must be a transaction UUID")
	h.mustFail("update_transaction", map[string]any{"id": "00000000-0000-4000-8000-000000000000", "amount": 1}, "transaction not found")
	h.mustFail("update_transaction", map[string]any{"id": exp.ID, "category": "Salary"}, "no expense category named")
	h.mustFail("update_transaction", map[string]any{"id": exp.ID, "category": "Salary"}, "Food")
	h.mustFail("update_transaction", map[string]any{"id": tr.ID, "category": "Food"}, "transfers cannot have a category")
	h.mustFail("update_transaction", map[string]any{"id": exp.ID, "to_account": "Cash"}, "only for transfers")
	h.mustFail("update_transaction", map[string]any{"id": exp.ID, "amount": -3}, "greater than zero")
	h.mustFail("update_transaction", map[string]any{"id": exp.ID, "account": "Nope"}, "available accounts")
	h.mustFail("update_transaction", map[string]any{"id": exp.ID, "date": "tomorrow"}, "occurred_on")
}

func TestUpdateTransactionsBatch(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.account("Card", "MXN")
	h.category("Food", "expense")
	h.category("Fun", "expense")
	a := h.addExpense(map[string]any{"amount": 10, "account": "Card", "category": "Food", "description": "A"})
	b := h.addExpense(map[string]any{"amount": 20, "account": "Card", "description": "B"})
	c := h.addExpense(map[string]any{"amount": 30, "account": "Card", "category": "Food", "description": "C"})

	var out transactionsOut
	text := h.mustCall("update_transactions", map[string]any{"items": []map[string]any{
		{"id": a.ID, "category": "Fun"},
		{"id": b.ID, "category": "Food", "amount": 21},
		{"id": c.ID, "clear_category": true},
	}}, &out)
	if len(out.Transactions) != 3 || out.Total != 3 || !strings.Contains(text, "Updated 3 transactions") {
		t.Fatalf("unexpected output: %+v / %s", out, text)
	}
	got := out.Transactions
	if got[0].ID != a.ID || got[0].Category != "Fun" || got[0].Amount != "10.00" || got[0].Description != "A" {
		t.Fatalf("unexpected first item: %+v", got[0])
	}
	if got[1].Category != "Food" || got[1].Amount != "21.00" {
		t.Fatalf("unexpected second item: %+v", got[1])
	}
	if got[2].Category != "" || got[2].Amount != "30.00" {
		t.Fatalf("unexpected third item: %+v", got[2])
	}
}

func TestUpdateTransactionsBatchRollsBack(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.account("Card", "MXN")
	h.category("Food", "expense")
	h.category("Fun", "expense")
	a := h.addExpense(map[string]any{"amount": 10, "account": "Card", "category": "Food"})
	b := h.addExpense(map[string]any{"amount": 20, "account": "Card", "category": "Food"})

	h.mustFail("update_transactions", map[string]any{"items": []map[string]any{
		{"id": a.ID, "category": "Fun"},
		{"id": b.ID, "category": "Missing"},
	}}, "nothing was updated: items[1]")

	// Fails inside the database transaction, after item 0 was applied.
	h.mustFail("update_transactions", map[string]any{"items": []map[string]any{
		{"id": a.ID, "category": "Fun"},
		{"id": "00000000-0000-4000-8000-000000000000", "category": "Fun"},
	}}, "items[1]")
	h.mustFail("update_transactions", map[string]any{"items": []map[string]any{
		{"id": a.ID, "category": "Fun"},
		{"id": a.ID, "amount": 3},
	}}, "items[1].id: duplicates items[0].id")

	var list transactionsOut
	h.mustCall("list_transactions", nil, &list)
	for _, tx := range list.Transactions {
		if tx.Category != "Food" {
			t.Fatalf("a failed batch must change nothing: %+v", tx)
		}
	}
}

func TestUpdateTransactionsBatchSize(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.mustFail("update_transactions", map[string]any{"items": []map[string]any{}}, "at least one item")

	items := make([]map[string]any, finance.MaxBatchSize+1)
	for i := range items {
		items[i] = map[string]any{"id": fmt.Sprintf("00000000-0000-4000-8000-%012d", i), "amount": 1}
	}
	h.mustFail("update_transactions", map[string]any{"items": items}, "at most 100 items")
}
