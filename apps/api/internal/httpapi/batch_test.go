package httpapi

import (
	"net/http"
	"strings"
	"testing"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/finance"
)

type batchBody[T any] struct {
	Items []T `json:"items"`
}

func listCategoriesCount(api *testAPI) int {
	var body batchBody[finance.Category]
	api.do(http.MethodGet, "/api/v1/categories?include_archived=true", nil).expect(http.StatusOK).decode(&body)
	return len(body.Items)
}

func listAccountsCount(api *testAPI) int {
	var body batchBody[finance.Account]
	api.do(http.MethodGet, "/api/v1/accounts?include_archived=true", nil).expect(http.StatusOK).decode(&body)
	return len(body.Items)
}

func listTransactionsTotal(api *testAPI) int64 {
	var page finance.TransactionPage
	api.do(http.MethodGet, "/api/v1/transactions", nil).expect(http.StatusOK).decode(&page)
	return page.Total
}

func assertDetail(t *testing.T, e errorBody, want string) {
	t.Helper()
	if !strings.Contains(e.Detail, want) {
		t.Fatalf("expected detail to contain %q, got %q (%+v)", want, e.Detail, e.Errors)
	}
}

func TestBatchRequiresAuth(t *testing.T) {
	t.Parallel()
	anon := newTestAPI(t).as("")
	for _, path := range []string{"/api/v1/categories/batch", "/api/v1/accounts/batch", "/api/v1/transactions/batch"} {
		anon.do(http.MethodPost, path, map[string]any{"items": []any{}}).expectError(http.StatusUnauthorized)
	}
}

func TestCreateCategoriesBatch(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)

	var out batchBody[finance.Category]
	api.do(http.MethodPost, "/api/v1/categories/batch", map[string]any{"items": []map[string]any{
		{"name": "Groceries", "kind": "expense", "color": "#22c55e"},
		{"name": "Salary", "kind": "income"},
		{"name": "Groceries", "kind": "income"},
	}}).expect(http.StatusCreated).decode(&out)
	if len(out.Items) != 3 || out.Items[0].Name != "Groceries" || out.Items[1].Kind != "income" || out.Items[2].Kind != "income" {
		t.Fatalf("unexpected categories: %+v", out.Items)
	}
	if got := listCategoriesCount(api); got != 3 {
		t.Fatalf("expected 3 categories, got %d", got)
	}
}

func TestCreateCategoriesBatchErrors(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	api.createCategory("Existing", "expense")
	url := "/api/v1/categories/batch"

	// Invalid item: points at index and field, and nothing is created.
	e := api.do(http.MethodPost, url, map[string]any{"items": []map[string]any{
		{"name": "A", "kind": "expense"},
		{"name": "B", "kind": "expense"},
		{"name": "C", "kind": "expense", "color": "green"},
	}}).expectError(http.StatusUnprocessableEntity)
	found := false
	for _, d := range e.Errors {
		found = found || strings.Contains(d.Location, "items[2].color")
	}
	if !found {
		t.Fatalf("error should point at items[2].color: %+v", e)
	}

	// Blank name is only caught by business rules.
	e = api.do(http.MethodPost, url, map[string]any{"items": []map[string]any{
		{"name": "A", "kind": "expense"},
		{"name": "   ", "kind": "expense"},
	}}).expectError(http.StatusUnprocessableEntity)
	assertDetail(t, e, "items[1].name: must not be empty")

	// Duplicate inside the batch, case-insensitive.
	e = api.do(http.MethodPost, url, map[string]any{"items": []map[string]any{
		{"name": "Fun", "kind": "expense"},
		{"name": "Other", "kind": "expense"},
		{"name": "fun", "kind": "expense"},
	}}).expectError(http.StatusUnprocessableEntity)
	assertDetail(t, e, "items[2].name: duplicates items[0].name")

	// Conflict with an existing category rolls back the earlier items.
	e = api.do(http.MethodPost, url, map[string]any{"items": []map[string]any{
		{"name": "New", "kind": "expense"},
		{"name": "existing", "kind": "expense"},
	}}).expectError(http.StatusConflict)
	assertDetail(t, e, "items[1]")
	assertDetail(t, e, "already exists")

	// Empty and oversized batches.
	api.do(http.MethodPost, url, map[string]any{"items": []map[string]any{}}).expectError(http.StatusUnprocessableEntity)
	tooMany := make([]map[string]any, finance.MaxBatchSize+1)
	for i := range tooMany {
		tooMany[i] = map[string]any{"name": "Cat " + strings.Repeat("x", i%5) + string(rune('a'+i%26)), "kind": "expense"}
	}
	api.do(http.MethodPost, url, map[string]any{"items": tooMany}).expectError(http.StatusUnprocessableEntity)

	if got := listCategoriesCount(api); got != 1 {
		t.Fatalf("failed batches must not create anything, got %d categories", got)
	}
}

func TestCreateCategoriesBatchMaxSize(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	items := make([]map[string]any, finance.MaxBatchSize)
	for i := range items {
		items[i] = map[string]any{"name": "Category " + strings.Repeat("a", i/26) + string(rune('a'+i%26)), "kind": "expense"}
	}
	var out batchBody[finance.Category]
	api.do(http.MethodPost, "/api/v1/categories/batch", map[string]any{"items": items}).expect(http.StatusCreated).decode(&out)
	if len(out.Items) != finance.MaxBatchSize {
		t.Fatalf("expected %d categories, got %d", finance.MaxBatchSize, len(out.Items))
	}
}

func TestCreateAccountsBatch(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)

	var out batchBody[finance.Account]
	api.do(http.MethodPost, "/api/v1/accounts/batch", map[string]any{"items": []map[string]any{
		{"name": "BBVA", "type": "checking", "currency": "mxn", "initial_balance": 150000},
		{"name": "Amex", "type": "credit_card", "currency": "USD", "initial_balance": -2500},
	}}).expect(http.StatusCreated).decode(&out)
	if len(out.Items) != 2 || out.Items[0].Currency != "MXN" || out.Items[0].Balance != 150000 || out.Items[1].Balance != -2500 {
		t.Fatalf("unexpected accounts: %+v", out.Items)
	}
}

func TestCreateAccountsBatchErrors(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	api.createAccount("Existing", "MXN", 0)
	url := "/api/v1/accounts/batch"

	e := api.do(http.MethodPost, url, map[string]any{"items": []map[string]any{
		{"name": "Ok", "type": "cash", "currency": "MXN"},
		{"name": "Bad", "type": "cash", "currency": "XXX"},
	}}).expectError(http.StatusUnprocessableEntity)
	assertDetail(t, e, "items[1].currency: unsupported currency")

	e = api.do(http.MethodPost, url, map[string]any{"items": []map[string]any{
		{"name": "Ok", "type": "cash", "currency": "MXN"},
		{"name": "OK", "type": "savings", "currency": "USD"},
	}}).expectError(http.StatusUnprocessableEntity)
	assertDetail(t, e, "items[1].name: duplicates items[0].name")

	e = api.do(http.MethodPost, url, map[string]any{"items": []map[string]any{
		{"name": "Fresh", "type": "cash", "currency": "MXN"},
		{"name": "EXISTING", "type": "cash", "currency": "MXN"},
	}}).expectError(http.StatusConflict)
	assertDetail(t, e, "items[1]")

	api.do(http.MethodPost, url, map[string]any{"items": []map[string]any{}}).expectError(http.StatusUnprocessableEntity)
	tooMany := make([]map[string]any, finance.MaxBatchSize+1)
	for i := range tooMany {
		tooMany[i] = map[string]any{"name": "Acc " + string(rune('a'+i%26)) + strings.Repeat("z", i/26), "type": "cash", "currency": "MXN"}
	}
	api.do(http.MethodPost, url, map[string]any{"items": tooMany}).expectError(http.StatusUnprocessableEntity)

	if got := listAccountsCount(api); got != 1 {
		t.Fatalf("failed batches must not create anything, got %d accounts", got)
	}
}

func TestCreateTransactionsBatch(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	mxn := api.createAccount("MXN", "MXN", 100000)
	usd := api.createAccount("USD", "USD", 0)
	food := api.createCategory("Food", "expense")
	pay := api.createCategory("Pay", "income")

	var out batchBody[finance.Transaction]
	api.do(http.MethodPost, "/api/v1/transactions/batch", map[string]any{"items": []map[string]any{
		{"type": "expense", "account_id": mxn.ID, "amount": 2500, "category_id": food.ID, "occurred_on": "2026-10-01"},
		{"type": "income", "account_id": mxn.ID, "amount": 50000, "category_id": pay.ID},
		{"type": "transfer", "account_id": mxn.ID, "amount": 20000, "destination_account_id": usd.ID, "destination_amount": 1000},
	}}).expect(http.StatusCreated).decode(&out)
	if len(out.Items) != 3 || out.Items[0].Type != "expense" || out.Items[1].Type != "income" || out.Items[2].Type != "transfer" {
		t.Fatalf("unexpected transactions: %+v", out.Items)
	}
	if got := api.getAccount(mxn.ID.String()).Balance; got != 100000-2500+50000-20000 {
		t.Fatalf("unexpected MXN balance %d", got)
	}
	if got := api.getAccount(usd.ID.String()).Balance; got != 1000 {
		t.Fatalf("unexpected USD balance %d", got)
	}
	if out.Items[0].CreatedBy == nil {
		t.Fatal("expected transactions to be attributed to the actor")
	}
}

func TestCreateTransactionsBatchErrors(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	mxn := api.createAccount("MXN", "MXN", 0)
	usd := api.createAccount("USD", "USD", 0)
	food := api.createCategory("Food", "expense")
	url := "/api/v1/transactions/batch"

	// The third item is invalid: the first two must be rolled back.
	e := api.do(http.MethodPost, url, map[string]any{"items": []map[string]any{
		{"type": "expense", "account_id": mxn.ID, "amount": 100},
		{"type": "expense", "account_id": mxn.ID, "amount": 200, "category_id": food.ID},
		{"type": "income", "account_id": mxn.ID, "amount": 300, "category_id": food.ID},
	}}).expectError(http.StatusUnprocessableEntity)
	assertDetail(t, e, "items[2].category_id")
	if got := listTransactionsTotal(api); got != 0 {
		t.Fatalf("expected rollback, got %d transactions", got)
	}
	if got := api.getAccount(mxn.ID.String()).Balance; got != 0 {
		t.Fatalf("balance changed after rollback: %d", got)
	}

	e = api.do(http.MethodPost, url, map[string]any{"items": []map[string]any{
		{"type": "expense", "account_id": mxn.ID, "amount": 100},
		{"type": "transfer", "account_id": mxn.ID, "amount": 100, "destination_account_id": usd.ID},
	}}).expectError(http.StatusUnprocessableEntity)
	assertDetail(t, e, "items[1].destination_amount")

	e = api.do(http.MethodPost, url, map[string]any{"items": []map[string]any{
		{"type": "expense", "account_id": missingID, "amount": 100},
	}}).expectError(http.StatusUnprocessableEntity)
	assertDetail(t, e, "items[0].account_id: account not found")

	api.do(http.MethodPost, url, map[string]any{"items": []map[string]any{}}).expectError(http.StatusUnprocessableEntity)
	tooMany := make([]map[string]any, finance.MaxBatchSize+1)
	for i := range tooMany {
		tooMany[i] = map[string]any{"type": "expense", "account_id": mxn.ID, "amount": 1}
	}
	api.do(http.MethodPost, url, map[string]any{"items": tooMany}).expectError(http.StatusUnprocessableEntity)
	if got := listTransactionsTotal(api); got != 0 {
		t.Fatalf("failed batches must not create anything, got %d", got)
	}

	// Exactly the maximum size is accepted.
	tooMany = tooMany[:finance.MaxBatchSize]
	api.do(http.MethodPost, url, map[string]any{"items": tooMany}).expect(http.StatusCreated)
	if got := listTransactionsTotal(api); got != finance.MaxBatchSize {
		t.Fatalf("expected %d transactions, got %d", finance.MaxBatchSize, got)
	}
}
