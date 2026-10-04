package httpapi

import (
	"net/http"
	"testing"
	"time"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/finance"
)

func TestSummary(t *testing.T) {
	t.Parallel()
	f := newTxFixture(t)
	transport := f.api.createCategory("Transport", "expense")

	for _, body := range []map[string]any{
		{"type": "income", "account_id": f.mxn.ID, "amount": 100000, "category_id": f.salary.ID, "occurred_on": "2026-09-01"},
		{"type": "expense", "account_id": f.mxn.ID, "amount": 3000, "category_id": f.food.ID, "occurred_on": "2026-09-05"},
		{"type": "expense", "account_id": f.mxn2.ID, "amount": 2000, "category_id": f.food.ID, "occurred_on": "2026-09-06"},
		{"type": "expense", "account_id": f.mxn.ID, "amount": 8000, "category_id": transport.ID, "occurred_on": "2026-09-07"},
		{"type": "expense", "account_id": f.mxn.ID, "amount": 500, "occurred_on": "2026-09-08"},
		{"type": "expense", "account_id": f.usd.ID, "amount": 1500, "category_id": f.food.ID, "occurred_on": "2026-09-09"},
		// Transfers never count as income or expense.
		{"type": "transfer", "account_id": f.mxn.ID, "amount": 17000, "destination_account_id": f.usd.ID, "destination_amount": 1000, "occurred_on": "2026-09-10"},
		// Outside the period.
		{"type": "expense", "account_id": f.mxn.ID, "amount": 99999, "category_id": f.food.ID, "occurred_on": "2026-10-01"},
	} {
		f.api.createTransaction(body)
	}

	var summary finance.Summary
	f.api.do(http.MethodGet, "/api/v1/summary?from=2026-09-01&to=2026-09-30", nil).expect(http.StatusOK).decode(&summary)

	if summary.From != "2026-09-01" || summary.To != "2026-09-30" || len(summary.Currencies) != 2 {
		t.Fatalf("unexpected summary: %+v", summary)
	}

	mxn, usd := summary.Currencies[0], summary.Currencies[1]
	if mxn.Currency != "MXN" || usd.Currency != "USD" {
		t.Fatalf("expected currencies sorted by code, got %s, %s", mxn.Currency, usd.Currency)
	}
	if mxn.Income != 100000 || mxn.Expense != 13500 || mxn.Net != 86500 {
		t.Fatalf("unexpected MXN totals: %+v", mxn)
	}
	// Balance is current (not limited to the period): includes the October expense and the transfer.
	if mxn.Balance != 100000-13500-17000-99999 {
		t.Fatalf("unexpected MXN balance: %d", mxn.Balance)
	}
	if len(mxn.Expenses) != 3 || *mxn.Expenses[0].CategoryName != "Transport" || mxn.Expenses[0].Total != 8000 {
		t.Fatalf("expected expenses sorted by total, got %+v", mxn.Expenses)
	}
	food := mxn.Expenses[1]
	if *food.CategoryName != "Food" || food.Total != 5000 || food.TransactionCount != 2 {
		t.Fatalf("unexpected food total: %+v", food)
	}
	if uncategorized := mxn.Expenses[2]; uncategorized.CategoryID != nil || uncategorized.Total != 500 {
		t.Fatalf("expected uncategorized bucket, got %+v", uncategorized)
	}
	if len(mxn.Incomes) != 1 || mxn.Incomes[0].Total != 100000 {
		t.Fatalf("unexpected incomes: %+v", mxn.Incomes)
	}

	if usd.Income != 0 || usd.Expense != 1500 || usd.Net != -1500 || usd.Balance != 1000-1500 {
		t.Fatalf("unexpected USD totals: %+v", usd)
	}
}

func TestSummaryDefaultsToCurrentMonth(t *testing.T) {
	t.Parallel()
	f := newTxFixture(t)
	f.api.createTransaction(map[string]any{"type": "expense", "account_id": f.mxn.ID, "amount": 700})

	var summary finance.Summary
	f.api.do(http.MethodGet, "/api/v1/summary", nil).expect(http.StatusOK).decode(&summary)

	now := time.Now().UTC()
	first := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	if summary.From != first.Format(time.DateOnly) || summary.To != first.AddDate(0, 1, -1).Format(time.DateOnly) {
		t.Fatalf("expected current month, got %s..%s", summary.From, summary.To)
	}
	// Currencies with active accounts appear even without activity.
	if len(summary.Currencies) != 2 || summary.Currencies[0].Expense != 700 {
		t.Fatalf("unexpected currencies: %+v", summary.Currencies)
	}
}

func TestSummaryValidation(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	api.do(http.MethodGet, "/api/v1/summary?from=2026-09-30&to=2026-09-01", nil).expectError(http.StatusUnprocessableEntity)
	api.do(http.MethodGet, "/api/v1/summary?from=september", nil).expect(http.StatusUnprocessableEntity)

	var empty finance.Summary
	api.do(http.MethodGet, "/api/v1/summary", nil).expect(http.StatusOK).decode(&empty)
	if len(empty.Currencies) != 0 {
		t.Fatalf("expected no currencies without accounts, got %+v", empty.Currencies)
	}
}
