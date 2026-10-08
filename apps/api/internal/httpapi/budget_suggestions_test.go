package httpapi

import (
	"net/http"
	"testing"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/finance"
)

func (a *testAPI) budgetSuggestions(query string) finance.BudgetSuggestions {
	a.t.Helper()
	var out finance.BudgetSuggestions
	a.do(http.MethodGet, "/api/v1/budgets/suggestions"+query, nil).expect(http.StatusOK).decode(&out)
	return out
}

// suggestionFor returns the suggestion of a category in a currency, or nil.
func suggestionFor(s finance.BudgetSuggestions, currency string, category finance.Category) *finance.BudgetSuggestion {
	for _, c := range s.Currencies {
		if c.Currency != currency {
			continue
		}
		for i := range c.Suggestions {
			if c.Suggestions[i].CategoryID == category.ID {
				return &c.Suggestions[i]
			}
		}
	}
	return nil
}

func expenseOn(account finance.Account, category finance.Category, amount int64, day string) map[string]any {
	return map[string]any{"type": "expense", "account_id": account.ID, "amount": amount, "category_id": category.ID, "occurred_on": day}
}

func TestBudgetSuggestionsMedian(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	mxn := api.createAccount("Card", "MXN", 0)
	usd := api.createAccount("Dollars", "USD", 0)
	food := api.createCategory("Food", "expense")
	gym := api.createCategory("Gym", "expense")
	old := api.createCategory("Old", "expense")
	salary := api.createCategory("Salary", "income")

	for _, body := range []map[string]any{
		// Food: 3 months, median is the middle one (20050 rounds up to 20100).
		expenseOn(mxn, food, 100, "2026-06-30"),
		expenseOn(mxn, food, 10000, "2026-07-01"),
		expenseOn(mxn, food, 20050, "2026-08-15"),
		expenseOn(mxn, food, 30000, "2026-09-30"),
		expenseOn(mxn, food, 90000, "2026-09-30"),
		// The target month itself is not a complete past month.
		expenseOn(mxn, food, 999999, "2026-10-01"),
		// Same category in another currency starts later: one month only.
		expenseOn(usd, food, 500, "2026-09-05"),
		// Gym started in August: July is skipped, September counts as zero.
		expenseOn(mxn, gym, 3000, "2026-08-03"),
		// Only spent before the window: no suggestion.
		expenseOn(mxn, old, 5000, "2026-06-10"),
		// Ignored: income and transfers.
		{"type": "income", "account_id": mxn.ID, "amount": 500000, "category_id": salary.ID, "occurred_on": "2026-09-01"},
		{"type": "transfer", "account_id": mxn.ID, "amount": 5000, "destination_account_id": usd.ID, "destination_amount": 300, "occurred_on": "2026-09-08"},
		// Ignored: uncategorized.
		{"type": "expense", "account_id": mxn.ID, "amount": 7000, "occurred_on": "2026-09-09"},
	} {
		api.createTransaction(body)
	}
	api.setBudgets("2026-09", budgetItem(food, "MXN", 15000))

	got := api.budgetSuggestions("?month=2026-10")
	if got.Month != "2026-10" || got.From != "2026-07" || got.To != "2026-09" {
		t.Fatalf("unexpected window: %+v", got)
	}
	if len(got.Currencies) != 2 || got.Currencies[0].Currency != "MXN" || got.Currencies[1].Currency != "USD" {
		t.Fatalf("expected MXN and USD only: %+v", got.Currencies)
	}

	f := suggestionFor(got, "MXN", food)
	if f == nil || f.Median != 20050 || f.Suggested != 20100 || f.Recurring != 0 || len(f.Months) != 3 {
		t.Fatalf("unexpected food suggestion: %+v", f)
	}
	if f.Months[0].Month != "2026-07" || f.Months[0].Spent != 10000 || f.Months[2].Spent != 120000 {
		t.Fatalf("unexpected months: %+v", f.Months)
	}
	if f.CurrentAmount == nil || *f.CurrentAmount != 15000 || *f.CurrentAmountMonth != "2026-09" {
		t.Fatalf("expected the budget in force: %+v", f)
	}

	g := suggestionFor(got, "MXN", gym)
	if g == nil || len(g.Months) != 2 || g.Months[0].Month != "2026-08" || g.Months[1].Spent != 0 || g.Median != 1500 || g.Suggested != 1500 || g.CurrentAmount != nil {
		t.Fatalf("unexpected gym suggestion: %+v", g)
	}

	u := suggestionFor(got, "USD", food)
	if u == nil || len(u.Months) != 1 || u.Median != 500 || u.Suggested != 500 {
		t.Fatalf("unexpected USD suggestion: %+v", u)
	}
	for _, c := range []finance.Category{old, salary} {
		if s := suggestionFor(got, "MXN", c); s != nil {
			t.Fatalf("%s should have no suggestion: %+v", c.Name, s)
		}
	}
	if n := len(got.Currencies[0].Suggestions); n != 2 {
		t.Fatalf("expected food and gym only in MXN, got %d", n)
	}
}

func TestBudgetSuggestionsRecurring(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	card := api.createAccount("Card", "MXN", 0)
	streaming := api.createCategory("Streaming", "expense")
	rent := api.createCategory("Rent", "expense")

	// A subscription created recently: only one unrelated purchase so far.
	api.createTransaction(expenseOn(card, streaming, 1000, "2026-09-02"))
	api.createRecurring(map[string]any{
		"name": "Video", "type": "expense", "account_id": card.ID, "category_id": streaming.ID, "amount": 1550,
		"interval_unit": "month", "start_on": "2026-09-20",
	})
	// A yearly one counts by its monthly equivalent.
	api.createRecurring(map[string]any{
		"name": "Cloud", "type": "expense", "account_id": card.ID, "category_id": streaming.ID, "amount": 12000,
		"interval_unit": "year", "start_on": "2026-09-25",
	})
	// Rent is already paid in the window: its cost is in the spending.
	api.createTransaction(expenseOn(card, rent, 8000, "2026-08-10"))
	paid := api.createTransaction(expenseOn(card, rent, 8000, "2026-09-10"))
	item := api.createRecurring(map[string]any{
		"name": "Rent", "type": "expense", "account_id": card.ID, "category_id": rent.ID, "amount": 8000,
		"interval_unit": "month", "start_on": "2026-08-10",
	})
	api.do(http.MethodPut, "/api/v1/transactions/"+paid.ID.String()+"/recurring", map[string]any{"recurring_id": item.ID}).expect(http.StatusOK)
	// Starts after the target month: not considered yet.
	api.createRecurring(map[string]any{
		"name": "Later", "type": "expense", "account_id": card.ID, "category_id": streaming.ID, "amount": 99900,
		"interval_unit": "month", "start_on": "2026-11-01",
	})

	got := api.budgetSuggestions("?month=2026-10")
	s := suggestionFor(got, "MXN", streaming)
	// 1000 median + 1550 + 12000/12 = 3550.
	if s == nil || s.Median != 1000 || s.Recurring != 2550 || s.Suggested != 3600 {
		t.Fatalf("unexpected streaming suggestion: %+v", s)
	}
	r := suggestionFor(got, "MXN", rent)
	if r == nil || r.Recurring != 0 || r.Median != 8000 || r.Suggested != 8000 {
		t.Fatalf("paid recurring must not be added twice: %+v", r)
	}
}

func TestBudgetSuggestionsEmptyAndArchived(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	if got := api.budgetSuggestions("?month=2026-10"); len(got.Currencies) != 0 {
		t.Fatalf("expected no suggestions: %+v", got)
	}
	// Defaults to the current month.
	if got := api.budgetSuggestions(""); got.Month == "" || len(got.Currencies) != 0 {
		t.Fatalf("unexpected default month: %+v", got)
	}

	card := api.createAccount("Card", "MXN", 0)
	old := api.createCategory("Old", "expense")
	api.createTransaction(expenseOn(card, old, 5000, "2026-09-10"))
	api.do(http.MethodPatch, "/api/v1/categories/"+old.ID.String(), map[string]any{"archived": true}).expect(http.StatusOK)
	if got := api.budgetSuggestions("?month=2026-10"); len(got.Currencies) != 0 {
		t.Fatalf("archived categories get no suggestion: %+v", got)
	}
}

func TestBudgetSuggestionsErrors(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	api.do(http.MethodGet, "/api/v1/budgets/suggestions?month=2026-13", nil).expect(http.StatusUnprocessableEntity)
	api.do(http.MethodGet, "/api/v1/budgets/suggestions?month=oct", nil).expect(http.StatusUnprocessableEntity)
	api.as("").do(http.MethodGet, "/api/v1/budgets/suggestions", nil).expect(http.StatusUnauthorized)
}
