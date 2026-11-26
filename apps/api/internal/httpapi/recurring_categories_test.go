package httpapi

import (
	"bytes"
	"net/http"
	"testing"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/finance"
)

func TestRecurringCategoryColorAndTotals(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	mxn := api.createAccount("Card", "MXN", 0)
	usd := api.createAccount("Dollars", "USD", 0)
	mkCategory := func(name, kind, color string) finance.Category {
		var c finance.Category
		body := map[string]any{"name": name, "kind": kind}
		if color != "" {
			body["color"] = color
		}
		api.do(http.MethodPost, "/api/v1/categories", body).expect(http.StatusCreated).decode(&c)
		return c
	}
	media := mkCategory("Media", "expense", "#ff0000")
	home := mkCategory("Home", "expense", "#00ff00")
	plain := mkCategory("Plain", "expense", "")
	salary := mkCategory("Salary", "income", "#0000ff")

	mk := func(name, kind string, account finance.Account, category *finance.Category, amount, count int, unit string) finance.RecurringItem {
		body := map[string]any{
			"name": name, "type": kind, "account_id": account.ID, "amount": amount,
			"interval_unit": unit, "interval_count": count, "start_on": "2099-01-01",
		}
		if category != nil {
			body["category_id"] = category.ID
		}
		return api.createRecurring(body)
	}

	var empty finance.RecurringSummary
	api.do(http.MethodGet, "/api/v1/recurring/summary", nil).expect(http.StatusOK).decode(&empty)

	netflix := mk("Netflix", "expense", mxn, &media, 20000, 1, "month")
	mk("Spotify", "expense", mxn, &media, 10000, 1, "month")
	mk("Insurance", "expense", mxn, &home, 120000, 1, "year") // 10000 / month
	mk("Cleaner", "expense", mxn, &home, 1000, 1, "week")     // 4333 / month
	mk("Misc", "expense", mxn, nil, 90000, 1, "month")        // uncategorized: biggest but last
	mk("Plain thing", "expense", mxn, &plain, 5000, 1, "month")
	mk("Salary", "income", mxn, &salary, 3000000, 1, "month")
	mk("Hosting", "expense", usd, nil, 3000, 3, "month") // 1000 / month
	paused := mk("Gym", "expense", mxn, &media, 50000, 1, "month")
	cancelled := mk("Old", "expense", mxn, &home, 70000, 1, "month")
	api.do(http.MethodPatch, "/api/v1/recurring/"+paused.ID.String(), map[string]any{"status": "paused"}).expect(http.StatusOK)
	api.do(http.MethodPatch, "/api/v1/recurring/"+cancelled.ID.String(), map[string]any{"status": "cancelled"}).expect(http.StatusOK)

	if netflix.CategoryColor == nil || *netflix.CategoryColor != "#ff0000" {
		t.Fatalf("expected category color on create, got %v", netflix.CategoryColor)
	}
	var got finance.RecurringItem
	api.do(http.MethodGet, "/api/v1/recurring/"+netflix.ID.String(), nil).expect(http.StatusOK).decode(&got)
	if got.CategoryColor == nil || *got.CategoryColor != "#ff0000" {
		t.Fatalf("expected category color on get, got %v", got.CategoryColor)
	}
	var list struct{ Items []finance.RecurringItem }
	api.do(http.MethodGet, "/api/v1/recurring", nil).expect(http.StatusOK).decode(&list)
	for _, it := range list.Items {
		switch it.Name {
		case "Misc", "Plain thing":
			if it.CategoryColor != nil {
				t.Fatalf("%s must have a null color: %v", it.Name, *it.CategoryColor)
			}
		case "Spotify":
			if it.CategoryColor == nil || *it.CategoryColor != "#ff0000" {
				t.Fatalf("expected color on list, got %v", it.CategoryColor)
			}
		}
	}

	type row struct {
		name  string
		color string
		exp   int64
		count int
	}
	check := func(cs finance.RecurringCurrencySummary, want []row) {
		t.Helper()
		if len(cs.Categories) != len(want) {
			t.Fatalf("%s: expected %d categories, got %+v", cs.Currency, len(want), cs.Categories)
		}
		var sum int64
		for i, w := range want {
			c := cs.Categories[i]
			name, color := "", ""
			if c.CategoryName != nil {
				name = *c.CategoryName
			}
			if c.CategoryColor != nil {
				color = *c.CategoryColor
			}
			if name != w.name || color != w.color || c.Expense != w.exp || c.Count != w.count ||
				(w.name == "" && c.CategoryID != nil) || (w.name != "" && c.CategoryID == nil) {
				t.Fatalf("%s: category %d: want %+v, got %+v", cs.Currency, i, w, c)
			}
			sum += c.Expense
		}
		if sum != cs.Expense {
			t.Fatalf("%s: categories sum %d != expense %d", cs.Currency, sum, cs.Expense)
		}
	}

	var summary finance.RecurringSummary
	api.do(http.MethodGet, "/api/v1/recurring/summary", nil).expect(http.StatusOK).decode(&summary)
	if len(summary.Currencies) != 2 {
		t.Fatalf("expected 2 currencies, got %+v", summary)
	}
	// Sorted by expense desc, uncategorized always last; paused, cancelled
	// and income items are excluded.
	check(summary.Currencies[0], []row{
		{"Media", "#ff0000", 30000, 2},
		{"Home", "#00ff00", 14333, 2},
		{"Plain", "", 5000, 1},
		{"", "", 90000, 1},
	})
	check(summary.Currencies[1], []row{{"", "", 1000, 1}})

	// Ties on expense are ordered by name.
	tieA := mkCategory("Alpha", "expense", "")
	tieB := mkCategory("Beta", "expense", "")
	mk("B item", "expense", usd, &tieB, 1000, 1, "month")
	mk("A item", "expense", usd, &tieA, 1000, 1, "month")
	api.do(http.MethodGet, "/api/v1/recurring/summary", nil).expect(http.StatusOK).decode(&summary)
	check(summary.Currencies[1], []row{{"Alpha", "", 1000, 1}, {"Beta", "", 1000, 1}, {"", "", 1000, 1}})
}

func TestRecurringSummaryCategoriesAreNeverNull(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	account := api.createAccount("Card", "MXN", 0)
	api.createRecurring(map[string]any{
		"name": "Salary", "type": "income", "account_id": account.ID, "amount": 1000,
		"interval_unit": "month", "start_on": "2099-01-01",
	})
	raw := api.do(http.MethodGet, "/api/v1/recurring/summary", nil).expect(http.StatusOK).Body
	if !bytes.Contains(raw, []byte(`"categories":[]`)) {
		t.Fatalf("expected an empty categories array for an income-only currency, got %s", raw)
	}
}
