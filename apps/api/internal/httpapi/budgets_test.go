package httpapi

import (
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/finance"
)

func (a *testAPI) budgetStatus(query string) finance.BudgetStatus {
	a.t.Helper()
	var status finance.BudgetStatus
	a.do(http.MethodGet, "/api/v1/budgets"+query, nil).expect(http.StatusOK).decode(&status)
	return status
}

func (a *testAPI) setBudgets(month string, items ...map[string]any) finance.BudgetStatus {
	a.t.Helper()
	var status finance.BudgetStatus
	a.do(http.MethodPut, "/api/v1/budgets/"+month, map[string]any{"items": items}).expect(http.StatusOK).decode(&status)
	return status
}

func budgetItem(category finance.Category, currency string, amount int64) map[string]any {
	return map[string]any{"category_id": category.ID, "currency": currency, "amount": amount}
}

// budgetLine returns the line of a category in a currency, failing the test
// when it is missing.
func budgetLine(t *testing.T, status finance.BudgetStatus, currency string, category finance.Category) finance.BudgetLine {
	t.Helper()
	for _, c := range status.Currencies {
		if c.Currency != currency {
			continue
		}
		for _, l := range c.Categories {
			if l.CategoryID != nil && *l.CategoryID == category.ID {
				return l
			}
		}
	}
	t.Fatalf("no %s line for %s in %+v", currency, category.Name, status)
	return finance.BudgetLine{}
}

func hasBudgetLine(status finance.BudgetStatus, currency string, category finance.Category) bool {
	for _, c := range status.Currencies {
		if c.Currency != currency {
			continue
		}
		for _, l := range c.Categories {
			if l.CategoryID != nil && *l.CategoryID == category.ID {
				return true
			}
		}
	}
	return false
}

func TestBudgetInheritance(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	account := api.createAccount("Card", "MXN", 0)
	food := api.createCategory("Food", "expense")

	// Set in January: later months inherit it.
	api.setBudgets("2026-01", budgetItem(food, "MXN", 10000))
	for _, month := range []string{"2026-01", "2026-03"} {
		line := budgetLine(t, api.budgetStatus("?month="+month), "MXN", food)
		if line.Amount == nil || *line.Amount != 10000 || line.AmountMonth == nil || *line.AmountMonth != "2026-01" {
			t.Fatalf("%s: expected the January amount, got %+v", month, line)
		}
	}
	// Earlier months have no budget at all.
	if hasBudgetLine(api.budgetStatus("?month=2025-12"), "MXN", food) {
		t.Fatal("December 2025 should have no budget line")
	}

	// Overriding February changes February and the months inheriting from it.
	api.setBudgets("2026-02", budgetItem(food, "MXN", 20000))
	feb := budgetLine(t, api.budgetStatus("?month=2026-02"), "MXN", food)
	mar := budgetLine(t, api.budgetStatus("?month=2026-03"), "MXN", food)
	jan := budgetLine(t, api.budgetStatus("?month=2026-01"), "MXN", food)
	if *feb.Amount != 20000 || *feb.AmountMonth != "2026-02" || *mar.Amount != 20000 || *mar.AmountMonth != "2026-02" || *jan.Amount != 10000 {
		t.Fatalf("unexpected amounts: jan=%+v feb=%+v mar=%+v", jan, feb, mar)
	}

	// Clearing April removes the budget from April on; spending still shows.
	api.createTransaction(map[string]any{"type": "expense", "account_id": account.ID, "amount": 700, "category_id": food.ID, "occurred_on": "2026-04-10"})
	api.setBudgets("2026-04", map[string]any{"category_id": food.ID, "currency": "MXN", "clear": true})
	apr := budgetLine(t, api.budgetStatus("?month=2026-04"), "MXN", food)
	may := api.budgetStatus("?month=2026-05")
	if apr.Amount != nil || apr.Remaining != nil || apr.State != "none" || apr.Spent != 700 {
		t.Fatalf("April should be unbudgeted with its spending: %+v", apr)
	}
	if hasBudgetLine(may, "MXN", food) {
		t.Fatalf("May inherits the cleared budget, expected no line: %+v", may)
	}
	if mar := budgetLine(t, api.budgetStatus("?month=2026-03"), "MXN", food); *mar.Amount != 20000 {
		t.Fatalf("March keeps its budget after clearing April: %+v", mar)
	}

	// A new amount after the clear brings the budget back.
	api.setBudgets("2026-06", budgetItem(food, "MXN", 5000))
	if jun := budgetLine(t, api.budgetStatus("?month=2026-07"), "MXN", food); *jun.Amount != 5000 || *jun.AmountMonth != "2026-06" {
		t.Fatalf("unexpected July budget: %+v", jun)
	}

	// Setting the same month again replaces the amount; zero is valid.
	status := api.setBudgets("2026-06", budgetItem(food, "MXN", 0))
	if line := budgetLine(t, status, "MXN", food); *line.Amount != 0 || line.State != "ok" || status.Month != "2026-06" {
		t.Fatalf("expected a zero budget, got %+v", line)
	}
}

func TestBudgetStatusFigures(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	mxn := api.createAccount("Card", "MXN", 0)
	usd := api.createAccount("Dollars", "USD", 0)
	food := api.createCategory("Food", "expense")
	fun := api.createCategory("Fun", "expense")
	rent := api.createCategory("Rent", "expense")
	salary := api.createCategory("Salary", "income")

	api.setBudgets("2026-09",
		budgetItem(food, "MXN", 10000),
		budgetItem(fun, "MXN", 10000),
		budgetItem(rent, "MXN", 10000),
		// The same category has an independent budget in another currency.
		budgetItem(food, "USD", 500),
	)
	for _, body := range []map[string]any{
		{"type": "expense", "account_id": mxn.ID, "amount": 7000, "category_id": food.ID, "occurred_on": "2026-09-02"},
		{"type": "expense", "account_id": mxn.ID, "amount": 1000, "category_id": food.ID, "occurred_on": "2026-09-30"},
		{"type": "expense", "account_id": mxn.ID, "amount": 2000, "category_id": fun.ID, "occurred_on": "2026-09-03"},
		{"type": "expense", "account_id": mxn.ID, "amount": 10001, "category_id": rent.ID, "occurred_on": "2026-09-04"},
		{"type": "expense", "account_id": usd.ID, "amount": 100, "category_id": food.ID, "occurred_on": "2026-09-05"},
		{"type": "expense", "account_id": mxn.ID, "amount": 300, "occurred_on": "2026-09-06"},
		// Ignored: income, transfers and other months.
		{"type": "income", "account_id": mxn.ID, "amount": 99999, "category_id": salary.ID, "occurred_on": "2026-09-07"},
		{"type": "transfer", "account_id": mxn.ID, "amount": 5000, "destination_account_id": usd.ID, "destination_amount": 300, "occurred_on": "2026-09-08"},
		{"type": "expense", "account_id": mxn.ID, "amount": 4000, "category_id": food.ID, "occurred_on": "2026-10-01"},
	} {
		api.createTransaction(body)
	}
	// Unbudgeted category with spending.
	extra := api.createCategory("Gifts", "expense")
	api.createTransaction(map[string]any{"type": "expense", "account_id": mxn.ID, "amount": 450, "category_id": extra.ID, "occurred_on": "2026-09-09"})

	status := api.budgetStatus("?month=2026-09")
	if status.Month != "2026-09" || len(status.Currencies) != 2 || status.Currencies[0].Currency != "MXN" || status.Currencies[1].Currency != "USD" {
		t.Fatalf("unexpected currencies: %+v", status)
	}

	cases := []struct {
		category                 finance.Category
		currency                 string
		amount, spent, remaining int64
		state                    string
	}{
		{food, "MXN", 10000, 8000, 2000, "near"},
		{fun, "MXN", 10000, 2000, 8000, "ok"},
		{rent, "MXN", 10000, 10001, -1, "over"},
		{food, "USD", 500, 100, 400, "ok"},
	}
	for _, c := range cases {
		l := budgetLine(t, status, c.currency, c.category)
		if *l.Amount != c.amount || l.Spent != c.spent || *l.Remaining != c.remaining || l.State != c.state || l.Committed != 0 {
			t.Fatalf("%s %s: unexpected line %+v", c.currency, c.category.Name, l)
		}
	}

	mxnStatus := status.Currencies[0]
	if gifts := budgetLine(t, status, "MXN", extra); gifts.Amount != nil || gifts.State != "none" || gifts.Spent != 450 {
		t.Fatalf("unexpected unbudgeted line: %+v", gifts)
	}
	if u := mxnStatus.Uncategorized; u == nil || u.Spent != 300 || u.State != "none" || u.CategoryID != nil {
		t.Fatalf("unexpected uncategorized bucket: %+v", u)
	}
	if status.Currencies[1].Uncategorized != nil {
		t.Fatalf("USD has no uncategorized spending: %+v", status.Currencies[1].Uncategorized)
	}
	// Totals only cover budgeted categories.
	if mxnStatus.Budgeted != 30000 || mxnStatus.Spent != 20001 || mxnStatus.Remaining != 9999 {
		t.Fatalf("unexpected MXN totals: %+v", mxnStatus)
	}
	names := []string{}
	for _, l := range mxnStatus.Categories {
		names = append(names, *l.CategoryName)
	}
	if len(names) != 4 || names[0] != "Food" || names[1] != "Fun" || names[2] != "Gifts" || names[3] != "Rent" {
		t.Fatalf("categories should be sorted by name: %v", names)
	}
}

func TestBudgetOwnerFilter(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	mine := api.createOwnedAccount("Mine", "me")
	shared := api.createOwnedAccount("Joint", "shared")
	food := api.createCategory("Food", "expense")
	api.setBudgets("2026-09", budgetItem(food, mine.Currency, 10000))
	api.createTransaction(map[string]any{"type": "expense", "account_id": mine.ID, "amount": 3000, "category_id": food.ID, "occurred_on": "2026-09-02"})
	api.createTransaction(map[string]any{"type": "expense", "account_id": shared.ID, "amount": 500, "category_id": food.ID, "occurred_on": "2026-09-03"})

	for query, spent := range map[string]int64{"": 3500, "&owner=me": 3000, "&owner=shared": 500} {
		l := budgetLine(t, api.budgetStatus("?month=2026-09"+query), mine.Currency, food)
		// The budget stays workspace-wide; only the spending narrows.
		if *l.Amount != 10000 || l.Spent != spent {
			t.Fatalf("owner filter %q: unexpected line %+v", query, l)
		}
	}
	api.do(http.MethodGet, "/api/v1/budgets?owner=nobody", nil).expect(http.StatusUnprocessableEntity)
}

func TestBudgetCommittedRecurring(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	account := api.createAccount("Card", "MXN", 0)
	rent := api.createCategory("Rent", "expense")
	today := time.Now().UTC()
	first := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, time.UTC)
	month := first.Format("2006-01")
	nextMonth := first.AddDate(0, 1, 0).Format("2006-01")

	api.setBudgets(month, budgetItem(rent, "MXN", 10000))
	item := api.createRecurring(map[string]any{
		"name": "Rent", "type": "expense", "account_id": account.ID, "category_id": rent.ID, "amount": 6000,
		"interval_unit": "month", "start_on": first.Format(time.DateOnly),
	})
	// An item of another category/state must not count.
	api.createRecurring(map[string]any{
		"name": "Salary", "type": "income", "account_id": account.ID, "amount": 90000,
		"interval_unit": "month", "start_on": first.Format(time.DateOnly),
	})

	line := budgetLine(t, api.budgetStatus("?month="+month), "MXN", rent)
	if line.Committed != 6000 || line.Spent != 0 || *line.Remaining != 4000 || line.State != "ok" {
		t.Fatalf("expected the unpaid rent as committed: %+v", line)
	}
	// Future months show their own committed due date.
	if next := budgetLine(t, api.budgetStatus("?month="+nextMonth), "MXN", rent); next.Committed != 6000 {
		t.Fatalf("expected next month's rent as committed: %+v", next)
	}
	// Months that already ended have no committed amount.
	api.setBudgets("2020-01", budgetItem(rent, "MXN", 10000))
	if past := budgetLine(t, api.budgetStatus("?month=2020-06"), "MXN", rent); past.Committed != 0 {
		t.Fatalf("past months have no committed amount: %+v", past)
	}
	// Committed plus spending can push the state to near/over.
	api.createTransaction(map[string]any{"type": "expense", "account_id": account.ID, "amount": 2500, "category_id": rent.ID, "occurred_on": first.Format(time.DateOnly)})
	if l := budgetLine(t, api.budgetStatus("?month="+month), "MXN", rent); l.Committed != 6000 || l.Spent != 2500 || *l.Remaining != 1500 || l.State != "near" {
		t.Fatalf("committed and spent add up: %+v", l)
	}

	// Registering the payment settles the period: it becomes spent and is no longer committed.
	api.do(http.MethodPost, "/api/v1/recurring/"+item.ID.String()+"/payments", map[string]any{"period": first.Format(time.DateOnly)}).expect(http.StatusCreated)
	l := budgetLine(t, api.budgetStatus("?month="+month), "MXN", rent)
	if l.Committed != 0 || l.Spent != 8500 || *l.Remaining != 1500 || l.State != "near" {
		t.Fatalf("a paid period is spent, not committed: %+v", l)
	}

	// Pausing the item removes its next occurrence.
	api.do(http.MethodPatch, "/api/v1/recurring/"+item.ID.String(), map[string]any{"status": "paused"}).expect(http.StatusOK)
	if next := budgetLine(t, api.budgetStatus("?month="+nextMonth), "MXN", rent); next.Committed != 0 {
		t.Fatalf("paused items are not committed: %+v", next)
	}
}

func TestBudgetCommittedRespectsOwner(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	shared := api.createOwnedAccount("Joint", "shared")
	food := api.createCategory("Food", "expense")
	first := time.Date(time.Now().UTC().Year(), time.Now().UTC().Month(), 1, 0, 0, 0, 0, time.UTC)
	month := first.Format("2006-01")
	api.setBudgets(month, budgetItem(food, shared.Currency, 10000))
	api.createRecurring(map[string]any{
		"name": "Groceries box", "type": "expense", "account_id": shared.ID, "category_id": food.ID, "amount": 3000,
		"interval_unit": "month", "start_on": first.Format(time.DateOnly),
	})
	if l := budgetLine(t, api.budgetStatus("?month="+month+"&owner=shared"), shared.Currency, food); l.Committed != 3000 {
		t.Fatalf("shared owner should see the commitment: %+v", l)
	}
	if l := budgetLine(t, api.budgetStatus("?month="+month+"&owner=me"), shared.Currency, food); l.Committed != 0 {
		t.Fatalf("my accounts have no commitment: %+v", l)
	}
}

func TestBudgetDefaultsToCurrentMonth(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	if status := api.budgetStatus(""); status.Month != time.Now().UTC().Format("2006-01") || len(status.Currencies) != 0 {
		t.Fatalf("unexpected default status: %+v", status)
	}
}

func TestBudgetImpact(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	account := api.createAccount("Card", "MXN", 0)
	food := api.createCategory("Food", "expense")
	other := api.createCategory("Other", "expense")
	api.setBudgets("2026-09", budgetItem(food, "MXN", 10000))
	api.createTransaction(map[string]any{"type": "expense", "account_id": account.ID, "amount": 5000, "category_id": food.ID, "occurred_on": "2026-09-02"})
	sept := time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC)

	cases := []struct {
		name          string
		amount        int64
		before, after string
		warning       bool
		remaining     int64
	}{
		{"stays ok", 1000, "ok", "ok", false, 4000},
		{"gets near", 3000, "ok", "near", true, 2000},
		{"reaches the limit", 5000, "ok", "near", true, 0},
		{"goes over", 5001, "ok", "over", true, -1},
	}
	for _, c := range cases {
		got, err := api.svc.BudgetImpact(api.ctx, food.ID, "mxn", sept, c.amount)
		if err != nil {
			t.Fatal(err)
		}
		if !got.HasBudget || got.StateBefore != c.before || got.StateAfter != c.after || got.Warning != c.warning || got.Remaining != c.remaining || got.Spent != 5000 || got.Month != "2026-09" {
			t.Fatalf("%s: unexpected impact %+v", c.name, got)
		}
	}

	// No budget (other category, other currency, earlier month): no warning.
	for name, args := range map[string]struct {
		category finance.Category
		currency string
		date     time.Time
	}{
		"no budget":      {other, "MXN", sept},
		"other currency": {food, "USD", sept},
		"earlier month":  {food, "MXN", time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC)},
	} {
		got, err := api.svc.BudgetImpact(api.ctx, args.category.ID, args.currency, args.date, 999999)
		if err != nil {
			t.Fatal(err)
		}
		if got.HasBudget || got.Warning || got.StateAfter != "none" {
			t.Fatalf("%s: expected no budget impact, got %+v", name, got)
		}
	}
	if _, err := api.svc.BudgetImpact(api.ctx, food.ID, "MXN", sept, -1); err == nil {
		t.Fatal("expected an error for a negative amount")
	}
}

func TestBudgetImpactEndpoint(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	account := api.createAccount("Card", "MXN", 0)
	food := api.createCategory("Food", "expense")
	other := api.createCategory("Other", "expense")
	api.setBudgets("2026-09", budgetItem(food, "MXN", 10000))
	api.createTransaction(map[string]any{"type": "expense", "account_id": account.ID, "amount": 5000, "category_id": food.ID, "occurred_on": "2026-09-02"})

	impact := func(category finance.Category, currency, date string, amount int) finance.BudgetImpactResult {
		var got finance.BudgetImpactResult
		query := "?category_id=" + category.ID.String() + "&currency=" + currency + "&date=" + date + "&amount=" + strconv.Itoa(amount)
		api.do(http.MethodGet, "/api/v1/budgets/impact"+query, nil).expect(http.StatusOK).decode(&got)
		return got
	}

	if got := impact(food, "MXN", "2026-09-15", 1000); !got.HasBudget || got.Warning || got.StateAfter != "ok" || got.Remaining != 4000 {
		t.Fatalf("expected an ok impact, got %+v", got)
	}
	if got := impact(food, "MXN", "2026-09-15", 3000); !got.Warning || got.StateAfter != "near" || got.StateBefore != "ok" {
		t.Fatalf("expected a near warning, got %+v", got)
	}
	if got := impact(food, "MXN", "2026-09-15", 6000); !got.Warning || got.StateAfter != "over" || got.Remaining != -1000 {
		t.Fatalf("expected an over warning, got %+v", got)
	}
	if got := impact(other, "MXN", "2026-09-15", 6000); got.HasBudget || got.Warning {
		t.Fatalf("a category without budget must not warn, got %+v", got)
	}

	base := "/api/v1/budgets/impact?category_id=" + food.ID.String() + "&currency=MXN&date=2026-09-15&amount=1"
	api.do(http.MethodGet, base, nil).expect(http.StatusOK)
	for name, query := range map[string]string{
		"bad date":         "?category_id=" + food.ID.String() + "&currency=MXN&date=2026-02-31&amount=1",
		"bad category":     "?category_id=nope&currency=MXN&date=2026-09-15&amount=1",
		"missing category": "?currency=MXN&date=2026-09-15&amount=1",
		"negative amount":  "?category_id=" + food.ID.String() + "&currency=MXN&date=2026-09-15&amount=-1",
		"bad currency":     "?category_id=" + food.ID.String() + "&currency=PESOS&date=2026-09-15&amount=1",
	} {
		if res := api.do(http.MethodGet, "/api/v1/budgets/impact"+query, nil); res.Status != http.StatusUnprocessableEntity && res.Status != http.StatusBadRequest {
			t.Fatalf("%s: expected a client error, got %d: %s", name, res.Status, res.Body)
		}
	}
	api.as("").do(http.MethodGet, base, nil).expect(http.StatusUnauthorized)
}

func TestSetBudgetsErrors(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	food := api.createCategory("Food", "expense")
	salary := api.createCategory("Salary", "income")
	api.setBudgets("2026-09", budgetItem(food, "MXN", 10000))

	missing := "00000000-0000-4000-8000-000000000000"
	cases := []struct {
		name  string
		month string
		items []map[string]any
	}{
		{"bad month", "2026-13", []map[string]any{budgetItem(food, "MXN", 1)}},
		{"month with a day", "2026-09-01", []map[string]any{budgetItem(food, "MXN", 1)}},
		{"income category", "2026-09", []map[string]any{budgetItem(salary, "MXN", 1)}},
		{"unknown category", "2026-09", []map[string]any{{"category_id": missing, "currency": "MXN", "amount": 1}}},
		{"unknown currency", "2026-09", []map[string]any{budgetItem(food, "ZZZ", 1)}},
		{"negative amount", "2026-09", []map[string]any{budgetItem(food, "MXN", -1)}},
		{"no amount nor clear", "2026-09", []map[string]any{{"category_id": food.ID, "currency": "MXN"}}},
		{"amount and clear", "2026-09", []map[string]any{{"category_id": food.ID, "currency": "MXN", "amount": 1, "clear": true}}},
		{"duplicated item", "2026-09", []map[string]any{budgetItem(food, "MXN", 1), budgetItem(food, "mxn", 2)}},
		{"empty items", "2026-09", []map[string]any{}},
	}
	for _, c := range cases {
		res := api.do(http.MethodPut, "/api/v1/budgets/"+c.month, map[string]any{"items": c.items})
		if res.Status != http.StatusUnprocessableEntity && res.Status != http.StatusBadRequest {
			t.Fatalf("%s: expected a client error, got %d: %s", c.name, res.Status, res.Body)
		}
	}

	// The error names the failing item and helps fixing it.
	res := api.do(http.MethodPut, "/api/v1/budgets/2026-09", map[string]any{"items": []map[string]any{budgetItem(food, "MXN", 1), budgetItem(salary, "MXN", 1)}})
	res.expect(http.StatusUnprocessableEntity)
	if body := string(res.Body); !strings.Contains(body, "items[1].category_id") || !strings.Contains(body, "income category") {
		t.Fatalf("expected a pointed error, got %s", body)
	}

	// Batches are atomic: the valid first item of a failing request is not applied.
	api.do(http.MethodPut, "/api/v1/budgets/2026-09", map[string]any{"items": []map[string]any{budgetItem(food, "MXN", 77), budgetItem(salary, "MXN", 1)}}).expect(http.StatusUnprocessableEntity)
	if l := budgetLine(t, api.budgetStatus("?month=2026-09"), "MXN", food); *l.Amount != 10000 {
		t.Fatalf("a failed batch must not change anything: %+v", l)
	}

	api.do(http.MethodGet, "/api/v1/budgets?month=2026-13", nil).expect(http.StatusUnprocessableEntity)
	api.do(http.MethodGet, "/api/v1/budgets?month=september", nil).expect(http.StatusUnprocessableEntity)
}

func TestBudgetsRequireAuth(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	anon := api.as("")
	anon.do(http.MethodGet, "/api/v1/budgets", nil).expect(http.StatusUnauthorized)
	anon.do(http.MethodPut, "/api/v1/budgets/2026-09", map[string]any{"items": []map[string]any{}}).expect(http.StatusUnauthorized)
}

func TestBudgetsAreIsolatedBetweenWorkspaces(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	account := api.createAccount("Secret card", "MXN", 0)
	food := api.createCategory("Secret food", "expense")
	api.setBudgets("2026-09", budgetItem(food, "MXN", 10000))
	api.createTransaction(map[string]any{"type": "expense", "account_id": account.ID, "amount": 500, "category_id": food.ID, "occurred_on": "2026-09-02"})

	other := api.newUser("other@example.com", "Other")
	otherFood := other.createCategory("Other food", "expense")
	if status := other.budgetStatus("?month=2026-09"); len(status.Currencies) != 0 {
		t.Fatalf("another workspace sees budgets: %+v", status)
	}
	// Another workspace's category can't be budgeted.
	res := other.do(http.MethodPut, "/api/v1/budgets/2026-09", map[string]any{"items": []map[string]any{budgetItem(food, "MXN", 1)}})
	res.expect(http.StatusUnprocessableEntity)

	// The same month has independent budgets per workspace.
	other.setBudgets("2026-09", budgetItem(otherFood, "MXN", 42))
	if l := budgetLine(t, api.budgetStatus("?month=2026-09"), "MXN", food); *l.Amount != 10000 || l.Spent != 500 {
		t.Fatalf("owner's budget changed: %+v", l)
	}
	mine := api.budgetStatus("?month=2026-09")
	if hasBudgetLine(mine, "MXN", otherFood) {
		t.Fatalf("owner sees the other workspace's budget: %+v", mine)
	}
}
