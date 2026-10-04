package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/finance"
)

// txFixture creates a common set of accounts and categories.
type txFixture struct {
	api                *testAPI
	mxn, mxn2, usd     finance.Account
	food, salary, gone finance.Category
}

func newTxFixture(t *testing.T) txFixture {
	t.Helper()
	api := newTestAPI(t)
	f := txFixture{
		api:    api,
		mxn:    api.createAccount("Checking", "MXN", 0),
		mxn2:   api.createAccount("Savings", "MXN", 0),
		usd:    api.createAccount("Dollars", "USD", 0),
		food:   api.createCategory("Food", "expense"),
		salary: api.createCategory("Salary", "income"),
		gone:   api.createCategory("Archived", "expense"),
	}
	api.do(http.MethodPatch, "/api/v1/categories/"+f.gone.ID.String(), map[string]any{"archived": true}).expect(http.StatusOK)
	return f
}

func TestCreateTransaction(t *testing.T) {
	t.Parallel()
	f := newTxFixture(t)

	expense := f.api.createTransaction(map[string]any{
		"type": "expense", "account_id": f.mxn.ID, "amount": 25050, "category_id": f.food.ID,
		"description": "  Supermarket ", "occurred_on": "2026-09-15",
	})
	if expense.Type != "expense" || expense.Amount != 25050 || expense.Currency != "MXN" || expense.MinorUnits != 2 {
		t.Fatalf("unexpected expense: %+v", expense)
	}
	if expense.Description != "Supermarket" || expense.OccurredOn != "2026-09-15" || *expense.CategoryName != "Food" {
		t.Fatalf("unexpected expense details: %+v", expense)
	}
	if expense.AccountName != "Checking" || expense.CreatedBy != nil {
		t.Fatalf("unexpected expense references: %+v", expense)
	}

	income := f.api.createTransaction(map[string]any{"type": "income", "account_id": f.mxn.ID, "amount": 100000, "category_id": f.salary.ID})
	if income.OccurredOn != time.Now().UTC().Format(time.DateOnly) {
		t.Fatalf("expected occurred_on to default to today, got %s", income.OccurredOn)
	}

	sameCurrency := f.api.createTransaction(map[string]any{"type": "transfer", "account_id": f.mxn.ID, "amount": 5000, "destination_account_id": f.mxn2.ID})
	if sameCurrency.DestinationAmount == nil || *sameCurrency.DestinationAmount != 5000 || *sameCurrency.DestinationAccountName != "Savings" {
		t.Fatalf("expected destination amount to default to amount: %+v", sameCurrency)
	}

	cross := f.api.createTransaction(map[string]any{
		"type": "transfer", "account_id": f.mxn.ID, "amount": 18000, "destination_account_id": f.usd.ID, "destination_amount": 1000,
	})
	if *cross.DestinationAmount != 1000 || *cross.DestinationCurrency != "USD" || *cross.DestinationMinorUnits != 2 {
		t.Fatalf("unexpected cross-currency transfer: %+v", cross)
	}
}

func TestCreateTransactionValidation(t *testing.T) {
	t.Parallel()
	f := newTxFixture(t)
	archived := f.api.createAccount("Closed", "MXN", 0)
	f.api.do(http.MethodPatch, "/api/v1/accounts/"+archived.ID.String(), map[string]any{"archived": true}).expect(http.StatusOK)

	tests := []struct {
		name  string
		body  map[string]any
		field string
	}{
		{"zero amount", map[string]any{"type": "expense", "account_id": f.mxn.ID, "amount": 0}, ""},
		{"unknown type", map[string]any{"type": "refund", "account_id": f.mxn.ID, "amount": 10}, ""},
		{"unknown account", map[string]any{"type": "expense", "account_id": missingID, "amount": 10}, "account_id"},
		{"archived account", map[string]any{"type": "expense", "account_id": archived.ID, "amount": 10}, "account_id"},
		{"unknown category", map[string]any{"type": "expense", "account_id": f.mxn.ID, "amount": 10, "category_id": missingID}, "category_id"},
		{"category kind mismatch", map[string]any{"type": "expense", "account_id": f.mxn.ID, "amount": 10, "category_id": f.salary.ID}, "category_id"},
		{"archived category", map[string]any{"type": "expense", "account_id": f.mxn.ID, "amount": 10, "category_id": f.gone.ID}, "category_id"},
		{"expense with destination", map[string]any{"type": "expense", "account_id": f.mxn.ID, "amount": 10, "destination_account_id": f.mxn2.ID}, "destination_account_id"},
		{"invalid date", map[string]any{"type": "expense", "account_id": f.mxn.ID, "amount": 10, "occurred_on": "15/09/2026"}, ""},
		{"transfer without destination", map[string]any{"type": "transfer", "account_id": f.mxn.ID, "amount": 10}, "destination_account_id"},
		{"transfer to same account", map[string]any{"type": "transfer", "account_id": f.mxn.ID, "amount": 10, "destination_account_id": f.mxn.ID}, "destination_account_id"},
		{"transfer to archived account", map[string]any{"type": "transfer", "account_id": f.mxn.ID, "amount": 10, "destination_account_id": archived.ID}, "destination_account_id"},
		{"transfer with category", map[string]any{"type": "transfer", "account_id": f.mxn.ID, "amount": 10, "destination_account_id": f.mxn2.ID, "category_id": f.food.ID}, "category_id"},
		{"cross-currency transfer without destination amount", map[string]any{"type": "transfer", "account_id": f.mxn.ID, "amount": 10, "destination_account_id": f.usd.ID}, "destination_amount"},
		{"same-currency transfer with different amounts", map[string]any{"type": "transfer", "account_id": f.mxn.ID, "amount": 10, "destination_account_id": f.mxn2.ID, "destination_amount": 11}, "destination_amount"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			body := f.api.do(http.MethodPost, "/api/v1/transactions", tc.body).expectError(http.StatusUnprocessableEntity)
			if tc.field == "" {
				return
			}
			if len(body.Errors) == 0 || body.Errors[0].Location != tc.field {
				t.Fatalf("expected error on %s, got %+v", tc.field, body)
			}
		})
	}
}

func TestCreateTransactionRecordsActor(t *testing.T) {
	t.Parallel()
	f := newTxFixture(t)
	ctx := context.Background()

	var userID uuid.UUID
	if err := f.api.pool.QueryRow(ctx, `INSERT INTO users (email, name) VALUES ('ana@example.com', 'Ana') RETURNING id`).Scan(&userID); err != nil {
		t.Fatal(err)
	}

	tx, err := f.api.svc.CreateTransaction(finance.WithActor(ctx, userID), finance.TransactionInput{
		Type: "expense", AccountID: f.mxn.ID, Amount: 100,
	})
	if err != nil {
		t.Fatal(err)
	}

	var got finance.Transaction
	f.api.do(http.MethodGet, "/api/v1/transactions/"+tx.ID.String(), nil).expect(http.StatusOK).decode(&got)
	if got.CreatedBy == nil || got.CreatedBy.ID != userID || got.CreatedBy.Name != "Ana" || got.CreatedBy.Email != "ana@example.com" {
		t.Fatalf("expected created_by to reference the actor, got %+v", got.CreatedBy)
	}
}

func TestGetTransaction(t *testing.T) {
	t.Parallel()
	f := newTxFixture(t)
	tx := f.api.createTransaction(map[string]any{"type": "expense", "account_id": f.mxn.ID, "amount": 10})

	var got finance.Transaction
	f.api.do(http.MethodGet, "/api/v1/transactions/"+tx.ID.String(), nil).expect(http.StatusOK).decode(&got)
	if got.ID != tx.ID {
		t.Fatalf("unexpected transaction: %+v", got)
	}
	f.api.do(http.MethodGet, "/api/v1/transactions/"+missingID, nil).expectError(http.StatusNotFound)
	f.api.do(http.MethodGet, "/api/v1/transactions/nope", nil).expect(http.StatusUnprocessableEntity)
}

func TestListTransactions(t *testing.T) {
	t.Parallel()
	f := newTxFixture(t)
	f.api.createTransaction(map[string]any{"type": "expense", "account_id": f.mxn.ID, "amount": 100, "category_id": f.food.ID, "description": "Tacos 100% al pastor", "occurred_on": "2026-09-01"})
	f.api.createTransaction(map[string]any{"type": "income", "account_id": f.mxn.ID, "amount": 900, "category_id": f.salary.ID, "occurred_on": "2026-09-10"})
	f.api.createTransaction(map[string]any{"type": "expense", "account_id": f.usd.ID, "amount": 50, "description": "Coffee", "occurred_on": "2026-09-20"})
	f.api.createTransaction(map[string]any{"type": "transfer", "account_id": f.mxn2.ID, "amount": 300, "destination_account_id": f.mxn.ID, "occurred_on": "2026-09-30"})

	list := func(query string) finance.TransactionPage {
		t.Helper()
		var page finance.TransactionPage
		f.api.do(http.MethodGet, "/api/v1/transactions"+query, nil).expect(http.StatusOK).decode(&page)
		return page
	}

	all := list("")
	if all.Total != 4 || len(all.Items) != 4 || all.Limit != finance.DefaultPageSize {
		t.Fatalf("unexpected page: total=%d items=%d limit=%d", all.Total, len(all.Items), all.Limit)
	}
	if all.Items[0].OccurredOn != "2026-09-30" || all.Items[3].OccurredOn != "2026-09-01" {
		t.Fatal("expected newest first")
	}

	tests := []struct {
		query string
		want  int64
	}{
		{"?account_id=" + f.mxn.ID.String(), 3}, // includes incoming transfer
		{"?category_id=" + f.food.ID.String(), 1},
		{"?type=expense", 2},
		{"?from=2026-09-10&to=2026-09-20", 2},
		{"?q=coffee", 1},
		{"?q=100%25", 1}, // "%" is matched literally
		{"?q=%25", 1},
		{"?type=transfer&account_id=" + f.mxn2.ID.String(), 1},
	}
	for _, tc := range tests {
		t.Run(tc.query, func(t *testing.T) {
			if got := list(tc.query).Total; got != tc.want {
				t.Fatalf("expected %d results, got %d", tc.want, got)
			}
		})
	}

	page := list("?limit=2&offset=2")
	if len(page.Items) != 2 || page.Total != 4 || page.Items[0].OccurredOn != "2026-09-10" {
		t.Fatalf("unexpected pagination: %+v", page)
	}

	for _, bad := range []string{"?account_id=nope", "?type=refund", "?from=yesterday", "?limit=500", "?offset=-1"} {
		t.Run("rejects "+bad, func(t *testing.T) {
			f.api.do(http.MethodGet, "/api/v1/transactions"+bad, nil).expect(http.StatusUnprocessableEntity)
		})
	}
}

func TestReplaceTransaction(t *testing.T) {
	t.Parallel()
	f := newTxFixture(t)
	tx := f.api.createTransaction(map[string]any{"type": "expense", "account_id": f.mxn.ID, "amount": 100, "category_id": f.food.ID, "occurred_on": "2026-09-01"})
	path := "/api/v1/transactions/" + tx.ID.String()

	var updated finance.Transaction
	f.api.do(http.MethodPut, path, map[string]any{
		"type": "transfer", "account_id": f.mxn.ID, "amount": 250, "destination_account_id": f.usd.ID,
		"destination_amount": 15, "description": "Converted", "occurred_on": "2026-09-02",
	}).expect(http.StatusOK).decode(&updated)
	if updated.Type != "transfer" || updated.Amount != 250 || *updated.DestinationAmount != 15 || updated.CategoryID != nil {
		t.Fatalf("unexpected replacement: %+v", updated)
	}
	if updated.OccurredOn != "2026-09-02" || updated.Description != "Converted" {
		t.Fatalf("unexpected replacement details: %+v", updated)
	}
	if got := f.api.getAccount(f.usd.ID.String()).Balance; got != 15 {
		t.Fatalf("expected destination balance 15, got %d", got)
	}

	f.api.do(http.MethodPut, path, map[string]any{"type": "transfer", "account_id": f.mxn.ID, "amount": 1}).
		expectError(http.StatusUnprocessableEntity)
	f.api.do(http.MethodPut, "/api/v1/transactions/"+missingID, map[string]any{"type": "expense", "account_id": f.mxn.ID, "amount": 1}).
		expectError(http.StatusNotFound)
}

func TestReplaceTransactionKeepsArchivedReferences(t *testing.T) {
	t.Parallel()
	f := newTxFixture(t)
	tx := f.api.createTransaction(map[string]any{"type": "expense", "account_id": f.mxn.ID, "amount": 100, "category_id": f.food.ID})
	f.api.do(http.MethodPatch, "/api/v1/accounts/"+f.mxn.ID.String(), map[string]any{"archived": true}).expect(http.StatusOK)
	f.api.do(http.MethodPatch, "/api/v1/categories/"+f.food.ID.String(), map[string]any{"archived": true}).expect(http.StatusOK)

	// Editing an old transaction must not fail because its account or
	// category was archived afterwards.
	f.api.do(http.MethodPut, "/api/v1/transactions/"+tx.ID.String(), map[string]any{
		"type": "expense", "account_id": f.mxn.ID, "amount": 120, "category_id": f.food.ID,
	}).expect(http.StatusOK)
}

func TestDeleteTransaction(t *testing.T) {
	t.Parallel()
	f := newTxFixture(t)
	tx := f.api.createTransaction(map[string]any{"type": "expense", "account_id": f.mxn.ID, "amount": 100})
	path := "/api/v1/transactions/" + tx.ID.String()

	f.api.do(http.MethodDelete, path, nil).expect(http.StatusNoContent)
	f.api.do(http.MethodGet, path, nil).expect(http.StatusNotFound)
	f.api.do(http.MethodDelete, path, nil).expectError(http.StatusNotFound)
	if got := f.api.getAccount(f.mxn.ID.String()).Balance; got != 0 {
		t.Fatalf("expected balance to be restored, got %d", got)
	}
}

func TestTransactionsRequireJSONBody(t *testing.T) {
	t.Parallel()
	f := newTxFixture(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/transactions", nil)
	rec := httptest.NewRecorder()
	f.api.handler.ServeHTTP(rec, req)
	if rec.Code < 400 || rec.Code >= 500 {
		t.Fatalf("expected a client error for an empty body, got %d", rec.Code)
	}
}
