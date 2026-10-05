package httpapi

import (
	"net/http"
	"testing"
	"time"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/finance"
)

func (a *testAPI) createRecurring(body map[string]any) finance.RecurringItem {
	a.t.Helper()
	var item finance.RecurringItem
	a.do(http.MethodPost, "/api/v1/recurring", body).expect(http.StatusCreated).decode(&item)
	return item
}

func TestCreateRecurringItem(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	account := api.createAccount("Card", "MXN", 0)
	entertainment := api.createCategory("Entertainment", "expense")

	item := api.createRecurring(map[string]any{
		"name": " Netflix ", "type": "expense", "account_id": account.ID, "category_id": entertainment.ID,
		"amount": 18900, "interval_unit": "month", "start_on": "2099-01-31", "notes": "family plan",
	})
	if item.Name != "Netflix" || item.Type != "expense" || item.Currency != "MXN" || item.MinorUnits != 2 ||
		item.AccountName != "Card" || item.CategoryName == nil || *item.CategoryName != "Entertainment" ||
		item.Amount != 18900 || item.MonthlyAmount != 18900 || item.IntervalCount != 1 ||
		item.Status != "active" || item.TotalPayments != nil || item.LastDueOn != nil || item.Notes != "family plan" {
		t.Fatalf("unexpected item: %+v", item)
	}
	if item.NextDueOn == nil || *item.NextDueOn != "2099-01-31" {
		t.Fatalf("expected next due on the start date, got %v", item.NextDueOn)
	}

	// Defaults: start today, every 1 unit.
	today := time.Now().UTC().Format(time.DateOnly)
	weekly := api.createRecurring(map[string]any{
		"name": "Cleaner", "type": "expense", "account_id": account.ID, "amount": 1200, "interval_unit": "week",
	})
	if weekly.StartOn == "" || weekly.IntervalCount != 1 || weekly.MonthlyAmount != 5200 {
		t.Fatalf("unexpected weekly item: %+v", weekly)
	}
	if weekly.StartOn != today && weekly.StartOn != time.Now().Add(-24*time.Hour).UTC().Format(time.DateOnly) {
		t.Fatalf("expected start_on to default to today, got %s", weekly.StartOn)
	}
}

func TestCreateRecurringItemInstallments(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	account := api.createAccount("Card", "MXN", 0)

	item := api.createRecurring(map[string]any{
		"name": "Phone", "type": "expense", "account_id": account.ID, "amount": 100000,
		"interval_unit": "month", "start_on": "2099-01-31", "total_payments": 12,
	})
	if item.TotalPayments == nil || *item.TotalPayments != 12 || item.LastDueOn == nil || *item.LastDueOn != "2099-12-31" {
		t.Fatalf("unexpected installment plan: %+v", item)
	}

	// A plan whose dues are all in the past has no next due date.
	old := api.createRecurring(map[string]any{
		"name": "Old plan", "type": "expense", "account_id": account.ID, "amount": 500,
		"interval_unit": "month", "start_on": "2020-01-15", "total_payments": 3,
	})
	if old.NextDueOn != nil || old.LastDueOn == nil || *old.LastDueOn != "2020-03-15" {
		t.Fatalf("unexpected exhausted plan: %+v", old)
	}
}

func TestCreateRecurringItemValidation(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	account := api.createAccount("Card", "MXN", 0)
	archived := api.createAccount("Old card", "MXN", 0)
	api.do(http.MethodPatch, "/api/v1/accounts/"+archived.ID.String(), map[string]any{"archived": true}).expect(http.StatusOK)
	income := api.createCategory("Salary", "income")
	archivedCategory := api.createCategory("Gone", "expense")
	api.do(http.MethodPatch, "/api/v1/categories/"+archivedCategory.ID.String(), map[string]any{"archived": true}).expect(http.StatusOK)

	base := func(overrides map[string]any) map[string]any {
		body := map[string]any{
			"name": "Gym", "type": "expense", "account_id": account.ID, "amount": 500, "interval_unit": "month",
		}
		for k, v := range overrides {
			body[k] = v
		}
		return body
	}
	tests := []struct {
		name string
		body map[string]any
	}{
		{"blank name", base(map[string]any{"name": "   "})},
		{"transfer type", base(map[string]any{"type": "transfer"})},
		{"zero amount", base(map[string]any{"amount": 0})},
		{"negative amount", base(map[string]any{"amount": -5})},
		{"amount too large", base(map[string]any{"amount": 2_000_000_000_000_000})},
		{"unknown unit", base(map[string]any{"interval_unit": "day"})},
		{"missing unit", base(map[string]any{"interval_unit": ""})},
		{"zero interval count", base(map[string]any{"interval_count": -1})},
		{"zero total payments", base(map[string]any{"total_payments": 0})},
		{"bad start date", base(map[string]any{"start_on": "01/02/2026"})},
		{"unknown account", base(map[string]any{"account_id": missingID})},
		{"archived account", base(map[string]any{"account_id": archived.ID})},
		{"category kind mismatch", base(map[string]any{"category_id": income.ID})},
		{"unknown category", base(map[string]any{"category_id": missingID})},
		{"archived category", base(map[string]any{"category_id": archivedCategory.ID})},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			api.do(http.MethodPost, "/api/v1/recurring", tc.body).expect(http.StatusUnprocessableEntity)
		})
	}

	body := api.do(http.MethodPost, "/api/v1/recurring", base(map[string]any{"category_id": income.ID})).
		expectError(http.StatusUnprocessableEntity)
	if body.Detail == "" {
		t.Fatalf("expected a problem detail, got %+v", body)
	}
}

func TestListAndGetRecurringItems(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	account := api.createAccount("Card", "MXN", 0)
	mk := func(name, kind string) finance.RecurringItem {
		return api.createRecurring(map[string]any{
			"name": name, "type": kind, "account_id": account.ID, "amount": 1000,
			"interval_unit": "month", "start_on": "2099-01-01",
		})
	}
	a := mk("Spotify", "expense")
	mk("Salary", "income")
	paused := mk("Gym", "expense")
	api.do(http.MethodPatch, "/api/v1/recurring/"+paused.ID.String(), map[string]any{"status": "paused"}).expect(http.StatusOK)

	var list struct{ Items []finance.RecurringItem }
	api.do(http.MethodGet, "/api/v1/recurring", nil).expect(http.StatusOK).decode(&list)
	if len(list.Items) != 3 || list.Items[2].Status != "paused" {
		t.Fatalf("expected 3 items with the paused one last, got %+v", list.Items)
	}
	if list.Items[2].NextDueOn != nil {
		t.Fatalf("paused items must not have a next due date: %+v", list.Items[2])
	}

	api.do(http.MethodGet, "/api/v1/recurring?status=paused", nil).expect(http.StatusOK).decode(&list)
	if len(list.Items) != 1 || list.Items[0].Name != "Gym" {
		t.Fatalf("unexpected paused items: %+v", list.Items)
	}
	api.do(http.MethodGet, "/api/v1/recurring?type=income", nil).expect(http.StatusOK).decode(&list)
	if len(list.Items) != 1 || list.Items[0].Name != "Salary" {
		t.Fatalf("unexpected income items: %+v", list.Items)
	}
	api.do(http.MethodGet, "/api/v1/recurring?status=bogus", nil).expect(http.StatusUnprocessableEntity)
	api.do(http.MethodGet, "/api/v1/recurring?type=transfer", nil).expect(http.StatusUnprocessableEntity)

	var got finance.RecurringItem
	api.do(http.MethodGet, "/api/v1/recurring/"+a.ID.String(), nil).expect(http.StatusOK).decode(&got)
	if got.ID != a.ID || got.Name != "Spotify" {
		t.Fatalf("unexpected item: %+v", got)
	}
	api.do(http.MethodGet, "/api/v1/recurring/"+missingID, nil).expectError(http.StatusNotFound)
	api.do(http.MethodGet, "/api/v1/recurring/123", nil).expect(http.StatusUnprocessableEntity)
}

func TestUpdateRecurringItem(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	account := api.createAccount("Card", "MXN", 0)
	media := api.createCategory("Media", "expense")
	salary := api.createCategory("Salary", "income")
	item := api.createRecurring(map[string]any{
		"name": "Netflix", "type": "expense", "account_id": account.ID, "amount": 18900,
		"interval_unit": "month", "start_on": "2099-01-15", "total_payments": 6,
	})
	path := "/api/v1/recurring/" + item.ID.String()

	var got finance.RecurringItem
	api.do(http.MethodPatch, path, map[string]any{
		"name": "Netflix 4K", "amount": 24000, "category_id": media.ID, "notes": "upgraded",
		"interval_unit": "year", "interval_count": 2, "start_on": "2099-02-01",
	}).expect(http.StatusOK).decode(&got)
	if got.Name != "Netflix 4K" || got.Amount != 24000 || got.CategoryID == nil || *got.CategoryID != media.ID ||
		got.IntervalUnit != "year" || got.IntervalCount != 2 || got.StartOn != "2099-02-01" || got.Notes != "upgraded" ||
		got.MonthlyAmount != 1000 || got.TotalPayments == nil || *got.TotalPayments != 6 || got.LastDueOn == nil || *got.LastDueOn != "2109-02-01" {
		t.Fatalf("unexpected item: %+v", got)
	}

	api.do(http.MethodPatch, path, map[string]any{"clear_category": true, "clear_total_payments": true}).
		expect(http.StatusOK).decode(&got)
	if got.CategoryID != nil || got.TotalPayments != nil || got.LastDueOn != nil {
		t.Fatalf("expected category and total cleared: %+v", got)
	}

	// Retire with status=cancelled; the item is kept and can still be read.
	api.do(http.MethodPatch, path, map[string]any{"status": "cancelled"}).expect(http.StatusOK).decode(&got)
	if got.Status != "cancelled" || got.NextDueOn != nil {
		t.Fatalf("unexpected cancelled item: %+v", got)
	}
	api.do(http.MethodGet, path, nil).expect(http.StatusOK)

	for name, body := range map[string]map[string]any{
		"kind mismatch":      {"category_id": salary.ID},
		"unknown category":   {"category_id": missingID},
		"category and clear": {"category_id": media.ID, "clear_category": true},
		"total and clear":    {"total_payments": 3, "clear_total_payments": true},
		"bad status":         {"status": "deleted"},
		"zero amount":        {"amount": 0},
		"blank name":         {"name": " "},
		"bad unit":           {"interval_unit": "day"},
		"bad count":          {"interval_count": 0},
		"bad start":          {"start_on": "nope"},
	} {
		t.Run(name, func(t *testing.T) {
			api.do(http.MethodPatch, path, body).expect(http.StatusUnprocessableEntity)
		})
	}
	api.do(http.MethodPatch, "/api/v1/recurring/"+missingID, map[string]any{"name": "X"}).expectError(http.StatusNotFound)
}

func TestRecurringItemsCannotBeDeleted(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	account := api.createAccount("Card", "MXN", 0)
	item := api.createRecurring(map[string]any{
		"name": "Gym", "type": "expense", "account_id": account.ID, "amount": 500, "interval_unit": "month",
	})
	api.do(http.MethodDelete, "/api/v1/recurring/"+item.ID.String(), nil).expect(http.StatusMethodNotAllowed)
}

func TestRecurringSummary(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	mxn := api.createAccount("Card", "MXN", 0)
	usd := api.createAccount("Dollars", "USD", 0)

	var summary finance.RecurringSummary
	api.do(http.MethodGet, "/api/v1/recurring/summary", nil).expect(http.StatusOK).decode(&summary)
	if len(summary.Currencies) != 0 {
		t.Fatalf("expected no currencies without items, got %+v", summary)
	}

	mk := func(name, kind string, account finance.Account, amount int, unit string, count int) finance.RecurringItem {
		return api.createRecurring(map[string]any{
			"name": name, "type": kind, "account_id": account.ID, "amount": amount,
			"interval_unit": unit, "interval_count": count, "start_on": "2099-01-01",
		})
	}
	mk("Netflix", "expense", mxn, 20000, "month", 1)
	mk("Insurance", "expense", mxn, 120000, "year", 1) // 10000 / month
	mk("Cleaner", "expense", mxn, 1000, "week", 1)     // 4333 / month
	mk("Salary", "income", mxn, 3000000, "month", 1)
	mk("Hosting", "expense", usd, 3000, "month", 3) // 1000 / month
	paused := mk("Gym", "expense", mxn, 50000, "month", 1)
	cancelled := mk("Old", "expense", mxn, 70000, "month", 1)
	api.do(http.MethodPatch, "/api/v1/recurring/"+paused.ID.String(), map[string]any{"status": "paused"}).expect(http.StatusOK)
	api.do(http.MethodPatch, "/api/v1/recurring/"+cancelled.ID.String(), map[string]any{"status": "cancelled"}).expect(http.StatusOK)

	api.do(http.MethodGet, "/api/v1/recurring/summary", nil).expect(http.StatusOK).decode(&summary)
	if len(summary.Currencies) != 2 {
		t.Fatalf("expected 2 currencies, got %+v", summary)
	}
	m, u := summary.Currencies[0], summary.Currencies[1]
	if m.Currency != "MXN" || m.MinorUnits != 2 || m.Expense != 34333 || m.Income != 3000000 || m.Net != 3000000-34333 ||
		m.ExpenseCount != 3 || m.IncomeCount != 1 {
		t.Fatalf("unexpected MXN summary: %+v", m)
	}
	if u.Currency != "USD" || u.Expense != 1000 || u.Income != 0 || u.Net != -1000 || u.ExpenseCount != 1 || u.IncomeCount != 0 {
		t.Fatalf("unexpected USD summary: %+v", u)
	}
}

func TestRecurringNamesAreUniqueAmongOpenItems(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	account := api.createAccount("Card", "MXN", 0)
	body := func(name string) map[string]any {
		return map[string]any{"name": name, "type": "expense", "account_id": account.ID, "amount": 500, "interval_unit": "month"}
	}
	netflix := api.createRecurring(body("Netflix"))
	other := api.createRecurring(body("Spotify"))

	// Create: case-insensitive clash.
	api.do(http.MethodPost, "/api/v1/recurring", body("netflix")).expect(http.StatusConflict)
	// Rename onto an existing name.
	path := "/api/v1/recurring/" + other.ID.String()
	api.do(http.MethodPatch, path, map[string]any{"name": "NETFLIX"}).expect(http.StatusConflict)
	// Renaming to its own name (different case) is fine.
	api.do(http.MethodPatch, path, map[string]any{"name": "SPOTIFY"}).expect(http.StatusOK)

	// A cancelled name can be reused, but the old item cannot be reactivated
	// while the new one is open.
	netflixPath := "/api/v1/recurring/" + netflix.ID.String()
	api.do(http.MethodPatch, netflixPath, map[string]any{"status": "cancelled"}).expect(http.StatusOK)
	api.createRecurring(body("Netflix"))
	api.do(http.MethodPatch, netflixPath, map[string]any{"status": "active"}).expect(http.StatusConflict)
}

func TestRecurringRequiresAuth(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	account := api.createAccount("Card", "MXN", 0)
	item := api.createRecurring(map[string]any{
		"name": "Gym", "type": "expense", "account_id": account.ID, "amount": 500, "interval_unit": "month",
	})
	anon := api.as("")
	body := map[string]any{"name": "X", "type": "expense", "account_id": account.ID, "amount": 1, "interval_unit": "month"}
	anon.do(http.MethodGet, "/api/v1/recurring", nil).expect(http.StatusUnauthorized)
	anon.do(http.MethodGet, "/api/v1/recurring/summary", nil).expect(http.StatusUnauthorized)
	anon.do(http.MethodPost, "/api/v1/recurring", body).expect(http.StatusUnauthorized)
	anon.do(http.MethodGet, "/api/v1/recurring/"+item.ID.String(), nil).expect(http.StatusUnauthorized)
	anon.do(http.MethodPatch, "/api/v1/recurring/"+item.ID.String(), map[string]any{"name": "X"}).expect(http.StatusUnauthorized)
}
