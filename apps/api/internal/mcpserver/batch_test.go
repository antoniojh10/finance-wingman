package mcpserver

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/finance"
)

func (h *harness) transactionCount() int64 {
	h.t.Helper()
	page, err := h.svc.ListTransactions(context.Background(), finance.TransactionFilter{})
	if err != nil {
		h.t.Fatal(err)
	}
	return page.Total
}

func TestCreateCategoriesBatch(t *testing.T) {
	t.Parallel()
	h := newHarness(t)

	var out categoriesOut
	text := h.mustCall("create_categories", map[string]any{"items": []map[string]any{
		{"name": "Groceries", "kind": "Expense", "color": "#22c55e"},
		{"name": "Salary", "kind": "income"},
		{"name": "Groceries", "kind": "income"},
	}}, &out)
	if len(out.Categories) != 3 || out.Categories[0].Kind != "expense" || out.Categories[2].Kind != "income" || !strings.Contains(text, "Created 3 categories") {
		t.Fatalf("unexpected output: %+v / %s", out, text)
	}
	cats, err := h.svc.ListCategories(context.Background(), nil, false)
	if err != nil || len(cats) != 3 {
		t.Fatalf("expected 3 categories: %+v %v", cats, err)
	}
}

func TestCreateCategoriesBatchErrors(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.category("Existing", "expense")

	h.mustFail("create_categories", map[string]any{"items": []map[string]any{
		{"name": "A", "kind": "expense"},
		{"name": "B", "kind": "refund"},
	}}, "items[1].kind: must be expense or income")
	h.mustFail("create_categories", map[string]any{"items": []map[string]any{
		{"name": "Fun", "kind": "expense"},
		{"name": "fun", "kind": "expense"},
	}}, "items[1].name: duplicates items[0].name")
	h.mustFail("create_categories", map[string]any{"items": []map[string]any{
		{"name": "New", "kind": "expense"},
		{"name": "existing", "kind": "expense"},
	}}, "items[1]: an active expense category with this name already exists")
	h.mustFail("create_categories", map[string]any{"items": []map[string]any{}}, "at least one item")

	items := make([]map[string]any, finance.MaxBatchSize+1)
	for i := range items {
		items[i] = map[string]any{"name": fmt.Sprintf("Cat %d", i), "kind": "expense"}
	}
	h.mustFail("create_categories", map[string]any{"items": items}, "at most 100 items")

	cats, err := h.svc.ListCategories(context.Background(), nil, false)
	if err != nil || len(cats) != 1 {
		t.Fatalf("failed batches must not create anything: %+v %v", cats, err)
	}
}

func TestCreateAccountsBatch(t *testing.T) {
	t.Parallel()
	h := newHarness(t)

	var out accountsOut
	text := h.mustCall("create_accounts", map[string]any{"items": []map[string]any{
		{"name": "BBVA", "type": "checking", "currency": "mxn", "initial_balance": 1500.5},
		{"name": "Amex", "type": "credit_card", "currency": "USD", "initial_balance": -250},
		{"name": "Wallet", "type": "cash", "currency": "JPY"},
	}}, &out)
	if len(out.Accounts) != 3 || out.Accounts[0].Balance != "1500.50" || out.Accounts[0].Currency != "MXN" || out.Accounts[1].Balance != "-250.00" || out.Accounts[2].Balance != "0" {
		t.Fatalf("unexpected accounts: %+v", out)
	}
	if !strings.Contains(text, "Created 3 accounts") {
		t.Fatalf("unexpected text: %s", text)
	}
}

func TestCreateAccountsBatchErrors(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.account("Existing", "MXN")

	h.mustFail("create_accounts", map[string]any{"items": []map[string]any{
		{"name": "Ok", "type": "cash", "currency": "MXN"},
		{"name": "Bad", "type": "cash", "currency": "XXX", "initial_balance": 10},
	}}, "items[1].currency")
	h.mustFail("create_accounts", map[string]any{"items": []map[string]any{
		{"name": "Ok", "type": "cash", "currency": "MXN"},
		{"name": "Bad", "type": "cash", "currency": "JPY", "initial_balance": 10.5},
	}}, "items[1]: initial_balance has too many decimals for JPY")
	h.mustFail("create_accounts", map[string]any{"items": []map[string]any{
		{"name": "Ok", "type": "cash", "currency": "MXN"},
		{"name": "Bad", "type": "piggy_bank", "currency": "MXN"},
	}}, "items[1].type: must be one of")
	h.mustFail("create_accounts", map[string]any{"items": []map[string]any{
		{"name": "Dup", "type": "cash", "currency": "MXN"},
		{"name": "dup", "type": "savings", "currency": "USD"},
	}}, "items[1].name: duplicates items[0].name")
	h.mustFail("create_accounts", map[string]any{"items": []map[string]any{
		{"name": "Fresh", "type": "cash", "currency": "MXN"},
		{"name": "EXISTING", "type": "cash", "currency": "MXN"},
	}}, "items[1]: an active account with this name already exists")
	h.mustFail("create_accounts", map[string]any{"items": []map[string]any{}}, "at least one item")

	items := make([]map[string]any, finance.MaxBatchSize+1)
	for i := range items {
		items[i] = map[string]any{"name": fmt.Sprintf("Acc %d", i), "type": "cash", "currency": "MXN"}
	}
	h.mustFail("create_accounts", map[string]any{"items": items}, "at most 100 items")

	accounts, err := h.svc.ListAccounts(context.Background(), true)
	if err != nil || len(accounts) != 1 {
		t.Fatalf("failed batches must not create anything: %+v %v", accounts, err)
	}
}

func TestAddTransactionsBatch(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.account("Checking", "MXN")
	h.account("Dollars", "USD")
	h.category("Food", "expense")
	h.category("Salary", "income")

	var out transactionsOut
	text := h.mustCall("add_transactions", map[string]any{"items": []map[string]any{
		{"type": "expense", "amount": 250.5, "account": "checking", "category": "food", "description": "Walmart", "date": "2026-09-15"},
		{"type": "income", "amount": 30000, "account": "Checking", "category": "Salary"},
		{"type": "transfer", "amount": 1700, "account": "Checking", "to_account": "Dollars", "destination_amount": 100},
	}}, &out)
	if len(out.Transactions) != 3 || out.Total != 3 || !strings.Contains(text, "Recorded 3 transactions") {
		t.Fatalf("unexpected output: %+v / %s", out, text)
	}
	if e := out.Transactions[0]; e.Type != "expense" || e.Amount != "250.50" || e.Category != "Food" || e.Date != "2026-09-15" {
		t.Fatalf("unexpected expense: %+v", e)
	}
	if tr := out.Transactions[2]; tr.Type != "transfer" || tr.DestinationAmount != "100.00" || tr.DestinationCurrency != "USD" {
		t.Fatalf("unexpected transfer: %+v", tr)
	}
	if got := h.transactionCount(); got != 3 {
		t.Fatalf("expected 3 transactions, got %d", got)
	}
}

func TestAddTransactionsBatchErrors(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.account("Checking", "MXN")
	h.account("Dollars", "USD")
	h.category("Food", "expense")
	h.category("Salary", "income")

	valid := map[string]any{"type": "expense", "amount": 10, "account": "Checking"}
	cases := []struct {
		name string
		item map[string]any
		want string
	}{
		{"unknown type", map[string]any{"type": "refund", "amount": 10, "account": "Checking"}, "items[1]: type must be expense, income or transfer"},
		{"unknown account lists valid ones", map[string]any{"type": "expense", "amount": 10, "account": "Nope"}, "items[1]: no account matches \"Nope\" for account; available accounts: Checking (MXN), Dollars (USD)"},
		{"missing account with several", map[string]any{"type": "expense", "amount": 10}, "items[1]: account is required because there are several accounts"},
		{"unknown category lists valid ones", map[string]any{"type": "expense", "amount": 10, "account": "Checking", "category": "Salary"}, "items[1]: no expense category named \"Salary\"; use one of: Food"},
		{"too many decimals", map[string]any{"type": "expense", "amount": 1.234, "account": "Checking"}, "items[1]: amount has too many decimals for MXN"},
		{"zero amount", map[string]any{"type": "expense", "amount": 0, "account": "Checking"}, "items[1]: amount must be greater than zero"},
		{"transfer without destination", map[string]any{"type": "transfer", "amount": 10, "account": "Checking"}, "items[1]: to_account is required"},
		{"transfer cross currency without amount", map[string]any{"type": "transfer", "amount": 10, "account": "Checking", "to_account": "Dollars"}, "items[1]: Checking uses MXN and Dollars uses USD"},
		{"transfer with category", map[string]any{"type": "transfer", "amount": 10, "account": "Checking", "to_account": "Dollars", "destination_amount": 1, "category": "Food"}, "items[1]: transfers cannot have a category"},
		{"expense with destination", map[string]any{"type": "expense", "amount": 10, "account": "Checking", "to_account": "Dollars"}, "items[1]: to_account and destination_amount are only for transfers"},
		{"bad date from business rules", map[string]any{"type": "expense", "amount": 10, "account": "Checking", "date": "yesterday"}, "items[1].occurred_on: must be a date in YYYY-MM-DD format"},
		{"same source and destination", map[string]any{"type": "transfer", "amount": 10, "account": "Checking", "to_account": "Checking"}, "items[1].destination_account_id: must be different"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h.mustFail("add_transactions", map[string]any{"items": []map[string]any{valid, tc.item}}, tc.want)
			h.mustFail("add_transactions", map[string]any{"items": []map[string]any{valid, tc.item}}, "nothing was created")
			if got := h.transactionCount(); got != 0 {
				t.Fatalf("expected rollback, got %d transactions", got)
			}
		})
	}

	h.mustFail("add_transactions", map[string]any{"items": []map[string]any{}}, "at least one item")
	items := make([]map[string]any, finance.MaxBatchSize+1)
	for i := range items {
		items[i] = valid
	}
	h.mustFail("add_transactions", map[string]any{"items": items}, "at most 100 items")
	if got := h.transactionCount(); got != 0 {
		t.Fatalf("failed batches must not create anything, got %d", got)
	}

	// Exactly the maximum size is accepted.
	var out transactionsOut
	h.mustCall("add_transactions", map[string]any{"items": items[:finance.MaxBatchSize]}, &out)
	if len(out.Transactions) != finance.MaxBatchSize {
		t.Fatalf("expected %d transactions, got %d", finance.MaxBatchSize, len(out.Transactions))
	}
}
