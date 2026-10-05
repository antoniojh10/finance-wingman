package mcpserver

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/finance"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/testutil"
)

type harness struct {
	t       *testing.T
	svc     *finance.Service
	session *mcp.ClientSession
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	ctx := context.Background()
	svc := finance.NewService(testutil.NewDatabase(t, true), time.UTC)
	server := New(svc, "test")

	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	if _, err := server.MCP().Connect(ctx, serverTransport, nil); err != nil {
		t.Fatal(err)
	}
	session, err := mcp.NewClient(&mcp.Implementation{Name: "test-client"}, nil).Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { session.Close() })
	return &harness{t: t, svc: svc, session: session}
}

func (h *harness) account(name, currency string) finance.Account {
	h.t.Helper()
	a, err := h.svc.CreateAccount(context.Background(), finance.CreateAccountInput{Name: name, Type: "checking", Currency: currency, BalanceAsOf: "2000-01-01"})
	if err != nil {
		h.t.Fatal(err)
	}
	return a
}

func (h *harness) category(name, kind string) finance.Category {
	h.t.Helper()
	c, err := h.svc.CreateCategory(context.Background(), finance.CreateCategoryInput{Name: name, Kind: kind})
	if err != nil {
		h.t.Fatal(err)
	}
	return c
}

// call invokes a tool and decodes its structured output into out (may be nil).
// It returns the text content and whether the tool reported an error.
func (h *harness) call(name string, args map[string]any, out any) (string, bool) {
	h.t.Helper()
	res, err := h.session.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		h.t.Fatalf("call %s: %v", name, err)
	}
	var text strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			text.WriteString(tc.Text)
		}
	}
	if out != nil && !res.IsError {
		raw, err := json.Marshal(res.StructuredContent)
		if err != nil {
			h.t.Fatal(err)
		}
		if err := json.Unmarshal(raw, out); err != nil {
			h.t.Fatalf("decode %s output: %v", name, err)
		}
	}
	return text.String(), res.IsError
}

func (h *harness) mustCall(name string, args map[string]any, out any) string {
	h.t.Helper()
	text, isErr := h.call(name, args, out)
	if isErr {
		h.t.Fatalf("%s returned an error: %s", name, text)
	}
	return text
}

func (h *harness) mustFail(name string, args map[string]any, contains string) {
	h.t.Helper()
	text, isErr := h.call(name, args, nil)
	if !isErr {
		h.t.Fatalf("%s should fail, got: %s", name, text)
	}
	if !strings.Contains(text, contains) {
		h.t.Fatalf("%s error %q should mention %q", name, text, contains)
	}
}

func TestToolsAreListed(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	res, err := h.session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]*mcp.Tool{}
	for _, tool := range res.Tools {
		got[tool.Name] = tool
	}
	for _, name := range []string{"add_expense", "add_income", "add_transfer", "list_accounts", "list_categories", "get_summary", "list_transactions", "delete_transaction", "create_account", "create_category", "create_categories", "create_accounts", "add_transactions", "update_account", "update_category", "list_recurring", "create_recurring", "update_recurring", "list_upcoming_recurring", "mark_recurring_paid", "list_recurring_suggestions", "accept_recurring_suggestion", "dismiss_recurring_suggestion", "link_transaction_to_recurring"} {
		if got[name] == nil {
			t.Errorf("missing tool %s", name)
		}
	}
	if !got["list_accounts"].Annotations.ReadOnlyHint || !*got["delete_transaction"].Annotations.DestructiveHint {
		t.Error("tool annotations are not set correctly")
	}
	if h.session.InitializeResult().Instructions == "" {
		t.Error("server instructions should be provided")
	}
}

func TestAddExpense(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.account("BBVA Checking", "MXN")
	h.category("Groceries", "expense")

	var out transactionOut
	text := h.mustCall("add_expense", map[string]any{
		"amount": 250.5, "account": "bbva checking", "category": "groceries", "description": "Walmart", "date": "2026-09-15",
	}, &out)
	if out.Amount != "250.50" || out.Currency != "MXN" || out.Category != "Groceries" || out.Date != "2026-09-15" || out.Description != "Walmart" {
		t.Fatalf("unexpected output: %+v", out)
	}
	if !strings.Contains(text, "250.50 MXN") || !strings.Contains(text, "BBVA Checking") {
		t.Fatalf("unexpected text: %s", text)
	}
}

func TestAddExpenseResolvesAccounts(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.mustFail("add_expense", map[string]any{"amount": 10}, "no active accounts")

	h.account("Cash", "MXN")
	var out transactionOut
	h.mustCall("add_expense", map[string]any{"amount": 10}, &out)
	if out.Account != "Cash" {
		t.Fatalf("single account should be used by default, got %+v", out)
	}

	h.account("Dollars", "USD")
	h.mustFail("add_expense", map[string]any{"amount": 10}, "Cash (MXN), Dollars (USD)")
	h.mustFail("add_expense", map[string]any{"amount": 10, "account": "Savings"}, `no account matches "Savings"`)

	// Unique partial matches are accepted.
	h.mustCall("add_expense", map[string]any{"amount": 3, "account": "dollar"}, &out)
	if out.Account != "Dollars" || out.Currency != "USD" {
		t.Fatalf("partial match failed: %+v", out)
	}
}

func TestAddExpenseValidation(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.account("Cash", "MXN")
	h.account("Yen", "JPY")
	h.category("Food", "expense")
	h.category("Salary", "income")

	h.mustFail("add_expense", map[string]any{"amount": 0, "account": "Cash"}, "greater than zero")
	h.mustFail("add_expense", map[string]any{"amount": -5, "account": "Cash"}, "greater than zero")
	h.mustFail("add_expense", map[string]any{"amount": 1.234, "account": "Cash"}, "too many decimals for MXN")
	h.mustFail("add_expense", map[string]any{"amount": 100.5, "account": "Yen"}, "too many decimals for JPY")
	h.mustFail("add_expense", map[string]any{"amount": 10, "account": "Cash", "category": "Salary"}, "use one of: Food")
	h.mustFail("add_expense", map[string]any{"amount": 10, "account": "Cash", "date": "yesterday"}, "YYYY-MM-DD")
}

func TestAddIncome(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.account("Payroll", "MXN")
	h.category("Salary", "income")

	var out transactionOut
	text := h.mustCall("add_income", map[string]any{"amount": 30000, "category": "Salary"}, &out)
	if out.Type != "income" || out.Amount != "30000.00" || out.Date != time.Now().UTC().Format(time.DateOnly) {
		t.Fatalf("unexpected income: %+v", out)
	}
	if !strings.HasPrefix(text, "Recorded income") {
		t.Fatalf("unexpected text: %s", text)
	}
}

func TestAddTransfer(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.account("Checking", "MXN")
	h.account("Savings", "MXN")
	h.account("Dollars", "USD")

	var out transactionOut
	h.mustCall("add_transfer", map[string]any{"amount": 500, "from_account": "Checking", "to_account": "Savings"}, &out)
	if out.Type != "transfer" || out.ToAccount != "Savings" || out.DestinationAmount != "500.00" {
		t.Fatalf("unexpected transfer: %+v", out)
	}

	h.mustFail("add_transfer", map[string]any{"amount": 1700, "from_account": "Checking", "to_account": "Dollars"}, "destination_amount")

	text := h.mustCall("add_transfer", map[string]any{"amount": 1700, "from_account": "Checking", "to_account": "Dollars", "destination_amount": 100}, &out)
	if out.DestinationAmount != "100.00" || out.DestinationCurrency != "USD" || !strings.Contains(text, "100.00 USD received") {
		t.Fatalf("unexpected cross-currency transfer: %+v / %s", out, text)
	}

	h.mustFail("add_transfer", map[string]any{"amount": 10, "from_account": "Checking", "to_account": "Checking"}, "different")
}

func TestListAccountsAndCategories(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	text := h.mustCall("list_accounts", nil, nil)
	if !strings.Contains(text, "no active accounts") {
		t.Fatalf("unexpected empty text: %s", text)
	}

	a := h.account("Cash", "MXN")
	if _, err := h.svc.CreateTransaction(context.Background(), finance.TransactionInput{Type: "income", AccountID: a.ID, Amount: 123456}); err != nil {
		t.Fatal(err)
	}
	var accounts accountsOut
	text = h.mustCall("list_accounts", nil, &accounts)
	if len(accounts.Accounts) != 1 || accounts.Accounts[0].Balance != "1234.56" || !strings.Contains(text, "1234.56 MXN") {
		t.Fatalf("unexpected accounts: %+v / %s", accounts, text)
	}

	h.category("Food", "expense")
	h.category("Salary", "income")
	var categories categoriesOut
	text = h.mustCall("list_categories", nil, &categories)
	if len(categories.Categories) != 2 || !strings.Contains(text, "Expense categories: Food") || !strings.Contains(text, "Income categories: Salary") {
		t.Fatalf("unexpected categories: %+v / %s", categories, text)
	}
	h.mustCall("list_categories", map[string]any{"kind": "income"}, &categories)
	if len(categories.Categories) != 1 {
		t.Fatalf("kind filter failed: %+v", categories)
	}
	h.mustFail("list_categories", map[string]any{"kind": "other"}, "expense or income")
}

func TestCreateAccount(t *testing.T) {
	t.Parallel()
	h := newHarness(t)

	var out accountOut
	text := h.mustCall("create_account", map[string]any{"name": "BBVA Checking", "type": "checking", "currency": "mxn", "initial_balance": 1500.5}, &out)
	if out.Name != "BBVA Checking" || out.Currency != "MXN" || out.Balance != "1500.50" || out.ID == "" {
		t.Fatalf("unexpected account: %+v", out)
	}
	if !strings.Contains(text, "Created checking account BBVA Checking in MXN with a balance of 1500.50 MXN") {
		t.Fatalf("unexpected text: %s", text)
	}

	h.mustCall("create_account", map[string]any{"name": "Amex", "type": "credit_card", "currency": "USD", "initial_balance": -250}, &out)
	if out.Balance != "-250.00" {
		t.Fatalf("negative opening balance: %+v", out)
	}
	h.mustCall("create_account", map[string]any{"name": "Wallet", "type": "cash", "currency": "JPY"}, &out)
	if out.Balance != "0" {
		t.Fatalf("default opening balance: %+v", out)
	}

	// The new account can be used right away.
	var tx transactionOut
	h.mustCall("add_expense", map[string]any{"amount": 100, "account": "BBVA"}, &tx)
	if tx.Account != "BBVA Checking" {
		t.Fatalf("expense on new account: %+v", tx)
	}

	h.mustFail("create_account", map[string]any{"name": "bbva checking", "type": "savings", "currency": "MXN"}, "already exists")
	h.mustFail("create_account", map[string]any{"name": "Piggy", "type": "piggy_bank", "currency": "MXN"}, "checking, savings, credit_card")
	h.mustFail("create_account", map[string]any{"name": "Piggy", "type": "cash", "currency": "XXX"}, "unsupported currency")
	h.mustFail("create_account", map[string]any{"name": "Piggy", "type": "cash", "currency": "XXX", "initial_balance": 10}, "ISO 4217")
	h.mustFail("create_account", map[string]any{"name": "Piggy", "type": "cash", "currency": "JPY", "initial_balance": 10.5}, "too many decimals for JPY")
	h.mustFail("create_account", map[string]any{"name": "  ", "type": "cash", "currency": "MXN"}, "name: must not be empty")
}

func TestCreateCategory(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.account("Cash", "MXN")

	var out categoryOut
	text := h.mustCall("create_category", map[string]any{"name": "Groceries", "kind": "Expense", "color": "#22c55e"}, &out)
	if out.Name != "Groceries" || out.Kind != "expense" || out.ID == "" || !strings.Contains(text, "Created expense category Groceries") {
		t.Fatalf("unexpected category: %+v / %s", out, text)
	}
	cat, err := h.svc.GetCategory(context.Background(), uuid.MustParse(out.ID))
	if err != nil || cat.Color == nil || *cat.Color != "#22c55e" {
		t.Fatalf("color not stored: %+v %v", cat, err)
	}

	// The new category can be used right away.
	var tx transactionOut
	h.mustCall("add_expense", map[string]any{"amount": 10, "category": "groceries"}, &tx)
	if tx.Category != "Groceries" {
		t.Fatalf("expense with new category: %+v", tx)
	}

	h.mustCall("create_category", map[string]any{"name": "Groceries", "kind": "income"}, &out)
	if out.Kind != "income" {
		t.Fatalf("same name in another kind should be allowed: %+v", out)
	}
	h.mustFail("create_category", map[string]any{"name": "groceries", "kind": "expense"}, "already exists")
	h.mustFail("create_category", map[string]any{"name": "Gifts", "kind": "refund"}, "expense or income")
	h.mustFail("create_category", map[string]any{"name": "Gifts", "kind": "expense", "color": "red"}, "hex color")
	h.mustFail("create_category", map[string]any{"name": "", "kind": "expense"}, "name: must not be empty")
}

func TestGetSummary(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.account("Cash", "MXN")
	h.category("Food", "expense")
	h.mustCall("add_income", map[string]any{"amount": 1000, "date": "2026-09-01"}, nil)
	h.mustCall("add_expense", map[string]any{"amount": 250.25, "category": "Food", "date": "2026-09-02"}, nil)
	h.mustCall("add_expense", map[string]any{"amount": 50, "date": "2026-09-03"}, nil)

	var out summaryOut
	text := h.mustCall("get_summary", map[string]any{"from": "2026-09-01", "to": "2026-09-30"}, &out)
	if len(out.Currencies) != 1 {
		t.Fatalf("unexpected summary: %+v", out)
	}
	mxn := out.Currencies[0]
	if mxn.Income != "1000.00" || mxn.Expense != "300.25" || mxn.Net != "699.75" || mxn.Balance != "699.75" {
		t.Fatalf("unexpected totals: %+v", mxn)
	}
	if len(mxn.Expenses) != 2 || mxn.Expenses[0].Category != "Food" || mxn.Expenses[1].Category != "Uncategorized" {
		t.Fatalf("unexpected breakdown: %+v", mxn.Expenses)
	}
	if !strings.Contains(text, "MXN — income 1000.00, expenses 300.25") {
		t.Fatalf("unexpected text: %s", text)
	}

	h.mustCall("get_summary", map[string]any{"period": "last_30_days"}, &out)
	h.mustFail("get_summary", map[string]any{"period": "forever"}, "unknown period")
}

func TestPeriodRange(t *testing.T) {
	today := time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)
	tests := map[string][2]string{
		"":             {"2026-03-01", "2026-03-31"},
		"this_month":   {"2026-03-01", "2026-03-31"},
		"last_month":   {"2026-02-01", "2026-02-28"},
		"this_year":    {"2026-01-01", "2026-12-31"},
		"last_30_days": {"2026-02-14", "2026-03-15"},
	}
	for period, want := range tests {
		from, to, err := periodRange(today, period)
		if err != nil || from != want[0] || to != want[1] {
			t.Errorf("periodRange(%q) = %s..%s (%v), want %s..%s", period, from, to, err, want[0], want[1])
		}
	}
}

func TestListTransactions(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.account("Cash", "MXN")
	h.account("Card", "MXN")
	h.category("Food", "expense")
	for i := 0; i < 12; i++ {
		h.mustCall("add_expense", map[string]any{"amount": i + 1, "account": "Cash", "description": "item"}, nil)
	}
	h.mustCall("add_expense", map[string]any{"amount": 99, "account": "Card", "category": "Food", "description": "Tacos"}, nil)

	var out transactionsOut
	text := h.mustCall("list_transactions", nil, &out)
	if len(out.Transactions) != 10 || out.Total != 13 || !strings.HasPrefix(text, "Showing 10 of 13") {
		t.Fatalf("default limit should be 10: %d/%d %s", len(out.Transactions), out.Total, text)
	}

	h.mustCall("list_transactions", map[string]any{"account": "Card"}, &out)
	if out.Total != 1 || out.Transactions[0].Description != "Tacos" {
		t.Fatalf("account filter: %+v", out)
	}
	h.mustCall("list_transactions", map[string]any{"category": "food"}, &out)
	if out.Total != 1 {
		t.Fatalf("category filter: %+v", out)
	}
	h.mustCall("list_transactions", map[string]any{"search": "taco", "limit": 100}, &out)
	if out.Total != 1 {
		t.Fatalf("search filter: %+v", out)
	}
	text = h.mustCall("list_transactions", map[string]any{"type": "income"}, &out)
	if out.Total != 0 || text != "No transactions match." {
		t.Fatalf("type filter: %+v %s", out, text)
	}
	h.mustFail("list_transactions", map[string]any{"type": "refund"}, "expense, income or transfer")
	h.mustFail("list_transactions", map[string]any{"account": "Nope"}, "no account matches")
}

func TestDeleteTransaction(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.account("Cash", "MXN")
	var created transactionOut
	h.mustCall("add_expense", map[string]any{"amount": 42}, &created)

	var out deleteOut
	text := h.mustCall("delete_transaction", map[string]any{"id": created.ID}, &out)
	if out.Deleted.ID != created.ID || !strings.Contains(text, "Deleted expense of 42.00 MXN") {
		t.Fatalf("unexpected delete result: %+v %s", out, text)
	}
	h.mustFail("delete_transaction", map[string]any{"id": created.ID}, "not found")
	h.mustFail("delete_transaction", map[string]any{"id": "abc"}, "UUID")
}

func TestUpdateAccount(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	a := h.account("BBVA", "MXN")
	h.account("Cash", "MXN")
	if _, err := h.svc.CreateTransaction(context.Background(), finance.TransactionInput{Type: "income", AccountID: a.ID, Amount: 10000}); err != nil {
		t.Fatal(err)
	}

	var out accountOut
	text := h.mustCall("update_account", map[string]any{"account": "bbva", "initial_balance": 500.25}, &out)
	if out.Balance != "600.25" || out.Name != "BBVA" || !strings.Contains(text, "balance is now 600.25 MXN") {
		t.Fatalf("balance fix: %+v / %s", out, text)
	}
	h.mustCall("update_account", map[string]any{"account": "BBVA", "name": "BBVA Checking", "type": "savings"}, &out)
	if out.Name != "BBVA Checking" || out.Type != "savings" || out.Balance != "600.25" {
		t.Fatalf("rename: %+v", out)
	}
	// History is kept.
	var txs transactionsOut
	h.mustCall("list_transactions", nil, &txs)
	if txs.Total != 1 {
		t.Fatalf("transactions lost: %+v", txs)
	}

	// Archive: hidden from list_accounts and unusable, but still editable.
	h.mustCall("update_account", map[string]any{"account": a.ID.String(), "archived": true}, &out)
	if !out.Archived {
		t.Fatalf("archive: %+v", out)
	}
	var accounts accountsOut
	h.mustCall("list_accounts", nil, &accounts)
	if len(accounts.Accounts) != 1 || accounts.Accounts[0].Name != "Cash" {
		t.Fatalf("archived account listed: %+v", accounts)
	}
	h.mustFail("add_expense", map[string]any{"amount": 1, "account": "BBVA Checking"}, "no account matches")
	out = accountOut{}
	h.mustCall("update_account", map[string]any{"account": "BBVA Checking", "archived": false}, &out)
	if out.Archived {
		t.Fatalf("unarchive: %+v", out)
	}
	h.mustCall("list_accounts", nil, &accounts)
	if len(accounts.Accounts) != 2 {
		t.Fatalf("restored account not listed: %+v", accounts)
	}
}

func TestUpdateAccountErrors(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.account("BBVA", "MXN")
	h.account("Cash", "MXN")

	h.mustFail("update_account", map[string]any{"account": "Nope", "name": "X"}, "available accounts: BBVA (MXN), Cash (MXN)")
	h.mustFail("update_account", map[string]any{"account": "BBVA", "name": "cash"}, "already exists")
	h.mustFail("update_account", map[string]any{"account": "BBVA", "type": "piggy_bank"}, "checking, savings, credit_card")
	h.mustFail("update_account", map[string]any{"account": "BBVA", "name": "  "}, "name: must not be empty")
	h.mustFail("update_account", map[string]any{"account": "BBVA", "initial_balance": 10.123}, "too many decimals for MXN")
	h.mustFail("update_account", map[string]any{"account": "BBVA"}, "nothing to update")
}

func TestUpdateCategory(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.category("Groceries", "expense")
	h.category("Salary", "income")

	var out categoryOut
	text := h.mustCall("update_category", map[string]any{"category": "groceries", "name": "Food"}, &out)
	if out.Name != "Food" || out.Kind != "expense" || !strings.Contains(text, "Updated expense category Food") {
		t.Fatalf("unexpected output: %+v / %s", out, text)
	}

	// Archive: hidden from list_categories but still editable
	h.mustCall("update_category", map[string]any{"category": "Food", "archived": true}, &out)
	if !out.Archived {
		t.Fatalf("archive: %+v", out)
	}
	var categories categoriesOut
	h.mustCall("list_categories", nil, &categories)
	if len(categories.Categories) != 1 || categories.Categories[0].Name != "Salary" {
		t.Fatalf("archived category listed: %+v", categories)
	}
	out = categoryOut{}
	h.mustCall("update_category", map[string]any{"category": "Food", "archived": false}, &out)
	if out.Archived {
		t.Fatalf("unarchive: %+v", out)
	}
	h.mustCall("list_categories", nil, &categories)
	if len(categories.Categories) != 2 {
		t.Fatalf("restored category not listed: %+v", categories)
	}
}

func TestUpdateCategoryErrors(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.category("Groceries", "expense")
	h.category("Food", "expense")

	h.mustFail("update_category", map[string]any{"category": "Nope", "name": "X"}, "no category matches")
	h.mustFail("update_category", map[string]any{"category": "Groceries", "name": "food"}, "already exists")
	h.mustFail("update_category", map[string]any{"category": "Groceries", "name": "  "}, "must not be empty")
	h.mustFail("update_category", map[string]any{"category": "Groceries", "color": "not-a-color"}, "hex color")
	h.mustFail("update_category", map[string]any{"category": "Groceries"}, "nothing to update")

	// The same name may exist for both kinds: ask for the id.
	h.category("Other", "expense")
	h.category("Other", "income")
	h.mustFail("update_category", map[string]any{"category": "Other", "name": "Misc"}, "pass the id instead")
}
