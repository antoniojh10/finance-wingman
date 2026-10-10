package httpapi

import (
	"net/http"
	"testing"
	"time"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/finance"
)

// monthsAgo returns the first day of the month n months before the current
// one, as a date and as a YYYY-MM label.
func monthsAgo(n int) (day, label string) {
	now := time.Now().UTC()
	first := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, -n, 0)
	return first.Format(time.DateOnly), first.Format("2006-01")
}

func monthLabel(n int) string {
	_, label := monthsAgo(n)
	return label
}

func (a *testAPI) monthlySummary(query string) finance.MonthlySummary {
	a.t.Helper()
	var out finance.MonthlySummary
	a.do(http.MethodGet, "/api/v1/summary/monthly"+query, nil).expect(http.StatusOK).decode(&out)
	return out
}

func monthlyCurrency(t *testing.T, s finance.MonthlySummary, code string) finance.MonthlyCurrency {
	t.Helper()
	for _, c := range s.Currencies {
		if c.Currency == code {
			return c
		}
	}
	t.Fatalf("no %s currency in %+v", code, s)
	return finance.MonthlyCurrency{}
}

func monthlyCategory(t *testing.T, c finance.MonthlyCurrency, name string) finance.MonthlyCategory {
	t.Helper()
	for _, cat := range c.Categories {
		if cat.Name != nil && *cat.Name == name {
			return cat
		}
	}
	t.Fatalf("no category %q in %+v", name, c.Categories)
	return finance.MonthlyCategory{}
}

func assertInts(t *testing.T, what string, got []int64, want ...int64) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: got %v, want %v", what, got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("%s: got %v, want %v", what, got, want)
		}
	}
}

// assertBudgets compares budgets with wanted values, where nil means null.
func assertBudgets(t *testing.T, what string, got []*int64, want ...any) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: got %d values, want %d", what, len(got), len(want))
	}
	for i, w := range want {
		switch {
		case w == nil && got[i] != nil:
			t.Fatalf("%s[%d]: got %d, want null", what, i, *got[i])
		case w != nil && (got[i] == nil || *got[i] != int64(w.(int))):
			t.Fatalf("%s[%d]: got %v, want %v", what, i, got[i], w)
		}
	}
}

func TestMonthlySummary(t *testing.T) {
	t.Parallel()
	f := newTxFixture(t)
	transport := f.api.createCategory("Transport", "expense")
	rent := f.api.createCategory("Rent", "expense")
	old := f.api.createCategory("Old", "expense")

	day := func(n int) string { d, _ := monthsAgo(n); return d }
	for _, body := range []map[string]any{
		{"type": "expense", "account_id": f.mxn.ID, "amount": 3000, "category_id": f.food.ID, "occurred_on": day(1)},
		{"type": "expense", "account_id": f.mxn.ID, "amount": 1500, "category_id": f.food.ID, "occurred_on": day(0)},
		{"type": "expense", "account_id": f.mxn2.ID, "amount": 500, "category_id": f.food.ID, "occurred_on": day(0)},
		{"type": "expense", "account_id": f.mxn.ID, "amount": 8000, "category_id": transport.ID, "occurred_on": day(3)},
		{"type": "expense", "account_id": f.mxn.ID, "amount": 700, "occurred_on": day(0)},
		{"type": "expense", "account_id": f.mxn.ID, "amount": 100, "category_id": old.ID, "occurred_on": day(0)},
		{"type": "expense", "account_id": f.usd.ID, "amount": 1500, "category_id": f.food.ID, "occurred_on": day(2)},
		// Income and transfers are not expenses.
		{"type": "income", "account_id": f.mxn.ID, "amount": 100000, "category_id": f.salary.ID, "occurred_on": day(1)},
		{"type": "transfer", "account_id": f.mxn.ID, "amount": 17000, "destination_account_id": f.usd.ID, "destination_amount": 1000, "occurred_on": day(1)},
		// Before the window.
		{"type": "expense", "account_id": f.mxn.ID, "amount": 99999, "category_id": f.food.ID, "occurred_on": day(6)},
	} {
		f.api.createTransaction(body)
	}

	f.api.do(http.MethodPatch, "/api/v1/categories/"+old.ID.String(), map[string]any{"archived": true}).expect(http.StatusOK)

	// Food: 5000 from four months ago, cleared a month ago. Transport:
	// 7000 from three months ago. Rent has a budget but no spending.
	f.api.setBudgets(monthLabel(4), budgetItem(f.food, "MXN", 5000))
	f.api.setBudgets(monthLabel(3), budgetItem(transport, "MXN", 7000))
	f.api.setBudgets(monthLabel(5), budgetItem(rent, "MXN", 100000))
	f.api.setBudgets(monthLabel(1), map[string]any{"category_id": f.food.ID, "currency": "MXN", "clear": true})

	s := f.api.monthlySummary("")
	if len(s.Months) != 6 || s.Months[5] != monthLabel(0) || s.Months[0] != monthLabel(5) {
		t.Fatalf("unexpected months: %v", s.Months)
	}
	if len(s.Currencies) != 2 {
		t.Fatalf("expected MXN and USD, got %+v", s.Currencies)
	}

	mxn := monthlyCurrency(t, s, "MXN")
	if mxn.MinorUnits != 2 || mxn.PartialMonth != monthLabel(0) || len(mxn.Months) != 6 {
		t.Fatalf("unexpected MXN header: %+v", mxn)
	}
	assertInts(t, "MXN totals", mxn.Totals, 0, 0, 8000, 0, 3000, 2800)
	// Categories ordered by period total: Transport 8000, Food 5000,
	// uncategorized 700, Archived 100.
	if len(mxn.Categories) != 4 {
		t.Fatalf("expected four categories, got %+v", mxn.Categories)
	}
	if mxn.Categories[0].Name == nil || *mxn.Categories[0].Name != "Transport" || *mxn.Categories[1].Name != "Food" {
		t.Fatalf("expected categories by period total, got %+v", mxn.Categories)
	}
	if u := mxn.Categories[2]; u.CategoryID != nil || u.Name != nil {
		t.Fatalf("expected the uncategorized row third, got %+v", u)
	}
	assertInts(t, "uncategorized", mxn.Categories[2].Totals, 0, 0, 0, 0, 0, 700)
	if a := mxn.Categories[3]; a.Name == nil || *a.Name != "Old" || !a.Archived {
		t.Fatalf("expected the archived category last, got %+v", a)
	}
	if monthlyCategory(t, mxn, "Food").Archived {
		t.Fatal("Food is not archived")
	}
	assertInts(t, "Transport", monthlyCategory(t, mxn, "Transport").Totals, 0, 0, 8000, 0, 0, 0)
	assertInts(t, "Food", monthlyCategory(t, mxn, "Food").Totals, 0, 0, 0, 0, 3000, 1500+500)

	// Food: no budget before month -4, inherited, cleared from last month.
	assertBudgets(t, "Food budgets", monthlyCategory(t, mxn, "Food").Budgets, nil, 5000, 5000, 5000, nil, nil)
	assertBudgets(t, "Transport budgets", monthlyCategory(t, mxn, "Transport").Budgets, nil, nil, 7000, 7000, 7000, 7000)
	assertBudgets(t, "uncategorized budgets", mxn.Categories[2].Budgets, nil, nil, nil, nil, nil, nil)
	// Totals include categories without spending (Rent) and skip cleared ones.
	assertBudgets(t, "MXN budgets", mxn.Budgets, 100000, 105000, 112000, 112000, 107000, 107000)

	usd := monthlyCurrency(t, s, "USD")
	assertInts(t, "USD totals", usd.Totals, 0, 0, 0, 1500, 0, 0)
	assertBudgets(t, "USD budgets", usd.Budgets, nil, nil, nil, nil, nil, nil)
}

func TestMonthlySummaryWindows(t *testing.T) {
	t.Parallel()
	f := newTxFixture(t)
	for _, n := range []int{0, 2, 5, 8, 11} {
		day, _ := monthsAgo(n)
		f.api.createTransaction(map[string]any{"type": "expense", "account_id": f.mxn.ID, "amount": 100 + n, "category_id": f.food.ID, "occurred_on": day})
	}

	for _, tc := range []struct {
		query  string
		months int
		totals []int64
	}{
		{"", 6, []int64{105, 0, 0, 102, 0, 100}},
		{"?months=3", 3, []int64{102, 0, 100}},
		{"?months=12", 12, []int64{111, 0, 0, 108, 0, 0, 105, 0, 0, 102, 0, 100}},
	} {
		s := f.api.monthlySummary(tc.query)
		if len(s.Months) != tc.months {
			t.Fatalf("%q: expected %d months, got %v", tc.query, tc.months, s.Months)
		}
		assertInts(t, tc.query+" totals", monthlyCurrency(t, s, "MXN").Totals, tc.totals...)
	}

	// Without expenses there is nothing to group.
	empty := newTestAPI(t).monthlySummary("")
	if len(empty.Currencies) != 0 || len(empty.Months) != 6 {
		t.Fatalf("unexpected empty summary: %+v", empty)
	}
}

func TestMonthlySummaryOwnerFilter(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	food := api.createCategory("Food", "expense")
	mine := api.createOwnedAccount("Mine", "me")
	shared := api.createOwnedAccount("Joint", "shared")
	day, label := monthsAgo(0)
	api.createTransaction(map[string]any{"type": "expense", "account_id": mine.ID, "amount": 300, "category_id": food.ID, "occurred_on": day})
	api.createTransaction(map[string]any{"type": "expense", "account_id": shared.ID, "amount": 200, "category_id": food.ID, "occurred_on": day})
	api.setBudgets(label, budgetItem(food, "EUR", 1000))

	all := monthlyCurrency(t, api.monthlySummary(""), "EUR")
	assertInts(t, "all totals", all.Totals, 0, 0, 0, 0, 0, 500)
	assertBudgets(t, "all budgets", all.Budgets, nil, nil, nil, nil, nil, 1000)

	for owner, want := range map[string]int64{"me": 300, "shared": 200} {
		eur := monthlyCurrency(t, api.monthlySummary("?owner="+owner), "EUR")
		assertInts(t, owner+" totals", eur.Totals, 0, 0, 0, 0, 0, want)
		// Budgets are workspace-wide: not reported for one owner.
		assertBudgets(t, owner+" budgets", eur.Budgets, nil, nil, nil, nil, nil, nil)
		assertBudgets(t, owner+" category budgets", eur.Categories[0].Budgets, nil, nil, nil, nil, nil, nil)
	}
	api.do(http.MethodGet, "/api/v1/summary/monthly?owner=Ana", nil).expectError(http.StatusUnprocessableEntity)
}

func TestMonthlySummaryErrors(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	for _, months := range []string{"0", "1", "4", "13", "-3", "abc"} {
		api.do(http.MethodGet, "/api/v1/summary/monthly?months="+months, nil).expectError(http.StatusUnprocessableEntity)
	}
	api.as("").do(http.MethodGet, "/api/v1/summary/monthly", nil).expectError(http.StatusUnauthorized)
}

func TestMonthlySummaryIsolatedByWorkspace(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	account := api.createAccount("Secret checking", "MXN", 0)
	category := api.createCategory("Secret food", "expense")
	day, label := monthsAgo(0)
	api.createTransaction(map[string]any{"type": "expense", "account_id": account.ID, "amount": 500, "category_id": category.ID, "occurred_on": day})
	api.setBudgets(label, budgetItem(category, "MXN", 900))

	other := api.newUser("other@example.com", "Other")
	other.createAccount("Other checking", "USD", 0)
	if s := other.monthlySummary("?months=12"); len(s.Currencies) != 0 {
		t.Fatalf("another workspace sees %+v", s.Currencies)
	}
}
