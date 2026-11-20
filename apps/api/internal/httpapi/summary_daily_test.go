package httpapi

import (
	"net/http"
	"net/url"
	"slices"
	"testing"
	"time"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/finance"
)

func (a *testAPI) dailySummary(query string) finance.DailySummary {
	a.t.Helper()
	var s finance.DailySummary
	a.do(http.MethodGet, "/api/v1/summary/daily?"+query, nil).expect(http.StatusOK).decode(&s)
	return s
}

func (a *testAPI) createTypedAccount(name, kind, currency string, initial int64, asOf string) finance.Account {
	a.t.Helper()
	var account finance.Account
	a.do(http.MethodPost, "/api/v1/accounts", map[string]any{
		"name": name, "type": kind, "currency": currency, "initial_balance": initial, "balance_as_of": asOf,
	}).expect(http.StatusCreated).decode(&account)
	return account
}

func rangeQuery(from, to int) string {
	return url.Values{"from": {daysFromToday(from)}, "to": {daysFromToday(to)}}.Encode()
}

func dayBalances(days []finance.DailyBalance) []int64 {
	out := make([]int64, len(days))
	for i, d := range days {
		out[i] = d.Balance
	}
	return out
}

func TestDailySummaryDefaultsToCurrentMonth(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	api.createAccount("Checking", "EUR", 1000)

	s := api.dailySummary("")
	now := time.Now().UTC()
	first := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	if s.From != first.Format(time.DateOnly) || s.To != first.AddDate(0, 1, -1).Format(time.DateOnly) || s.Today != daysFromToday(0) {
		t.Fatalf("unexpected range %s..%s today %s", s.From, s.To, s.Today)
	}
	if len(s.Currencies) != 1 || len(s.Currencies[0].Days) != now.Day() {
		t.Fatalf("expected one entry per day up to today: %+v", s.Currencies)
	}
	if last := s.Currencies[0].Days[now.Day()-1]; last.Date != daysFromToday(0) || last.Balance != 1000 {
		t.Fatalf("unexpected last day: %+v", last)
	}
}

func TestDailySummaryBalancesDayByDay(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	salary := api.createCategory("Salary", "income")
	food := api.createCategory("Food", "expense")
	checking := api.createAccount("Checking", "EUR", 10000)
	savings := api.createTypedAccount("Savings", "savings", "EUR", 500, "2000-01-01")
	invest := api.createTypedAccount("Broker", "investment", "EUR", 7000, "2000-01-01")

	api.createTransaction(map[string]any{"type": "income", "account_id": checking.ID, "amount": 5000, "category_id": salary.ID, "occurred_on": daysFromToday(-3)})
	api.createTransaction(map[string]any{"type": "expense", "account_id": checking.ID, "amount": 1000, "category_id": food.ID, "occurred_on": daysFromToday(-2)})
	api.createTransaction(map[string]any{"type": "transfer", "account_id": checking.ID, "amount": 2000, "destination_account_id": savings.ID, "occurred_on": daysFromToday(-1)})
	api.createTransaction(map[string]any{"type": "transfer", "account_id": checking.ID, "amount": 100, "destination_account_id": invest.ID, "occurred_on": daysFromToday(0)})

	s := api.dailySummary(rangeQuery(-4, 0))
	if len(s.Currencies) != 1 || len(s.Currencies[0].Days) != 5 {
		t.Fatalf("unexpected currencies: %+v", s.Currencies)
	}
	c := s.Currencies[0]
	if c.Currency != "EUR" || c.MinorUnits != 2 || len(c.Projection) != 0 {
		t.Fatalf("unexpected currency: %+v", c)
	}
	if got, want := dayBalances(c.Days), []int64{10000, 15000, 14000, 12000, 11900}; !slices.Equal(got, want) {
		t.Fatalf("balances %v, want %v", got, want)
	}
	savingsWant := []int64{500, 500, 500, 2500, 2500}
	for i, d := range c.Days {
		if d.SavingsBalance != savingsWant[i] || d.InvestmentBalance != map[bool]int64{true: 7000, false: 7100}[i < 4] {
			t.Fatalf("day %d buckets: %+v", i, d)
		}
	}
	if c.Days[1].Income != 5000 || c.Days[1].Expense != 0 || c.Days[2].Expense != 1000 || c.Days[3].Income != 0 || c.Days[3].Expense != 0 {
		t.Fatalf("income/expense must skip transfers: %+v", c.Days)
	}

	if len(s.Accounts) != 3 || s.Accounts[0].Name != "Broker" || s.Accounts[1].Name != "Checking" || s.Accounts[2].Name != "Savings" {
		t.Fatalf("unexpected accounts: %+v", s.Accounts)
	}
	checkingDays := s.Accounts[1].Days
	if len(checkingDays) != 5 || checkingDays[4].Balance != 11900 || checkingDays[0].Date != daysFromToday(-4) {
		t.Fatalf("unexpected checking days: %+v", checkingDays)
	}

	// Today's entry matches /summary.
	var summary finance.Summary
	api.do(http.MethodGet, "/api/v1/summary", nil).expect(http.StatusOK).decode(&summary)
	today := c.Days[4]
	got := summary.Currencies[0]
	if got.Balance != today.Balance || got.SavingsBalance != today.SavingsBalance || got.InvestmentBalance != today.InvestmentBalance {
		t.Fatalf("today %+v differs from summary %+v", today, got)
	}
}

func TestDailySummaryBalanceAnchorInsideRange(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	account := api.createTypedAccount("Checking", "checking", "EUR", 10000, daysFromToday(-3))
	// Backdated before the anchor: already inside the initial balance.
	api.createTransaction(map[string]any{"type": "income", "account_id": account.ID, "amount": 300, "occurred_on": daysFromToday(-5)})
	api.createTransaction(map[string]any{"type": "expense", "account_id": account.ID, "amount": 1000, "occurred_on": daysFromToday(-2)})

	s := api.dailySummary(rangeQuery(-5, 0))
	if got, want := dayBalances(s.Currencies[0].Days), []int64{10000, 10000, 10000, 9000, 9000, 9000}; !slices.Equal(got, want) {
		t.Fatalf("balances %v, want %v", got, want)
	}
	if current := api.getAccount(account.ID.String()).Balance; current != 9000 {
		t.Fatalf("account balance %d", current)
	}
}

func TestDailySummaryFutureRangeHasNoDays(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	account := api.createAccount("Checking", "EUR", 50000)
	api.createTransaction(map[string]any{"type": "expense", "account_id": account.ID, "amount": 3000, "occurred_on": daysFromToday(-5)})

	s := api.dailySummary(rangeQuery(3, 5))
	c := s.Currencies[0]
	if len(c.Days) != 0 || len(s.Accounts[0].Days) != 0 {
		t.Fatalf("expected no days: %+v", c.Days)
	}
	// 3000 / 30 = 100 per day from the end-of-today balance 47000.
	if len(c.Projection) != 3 || c.Projection[0].Date != daysFromToday(3) || c.Projection[0].Balance != 47000-300 || c.Projection[2].Balance != 47000-500 {
		t.Fatalf("unexpected projection: %+v", c.Projection)
	}
}

func TestDailySummaryProjection(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	checking := api.createAccount("Checking", "EUR", 100000)
	savings := api.createTypedAccount("Savings", "savings", "EUR", 0, "2000-01-01")
	food := api.createCategory("Food", "expense")

	// Spending in the window (3000 / 30 = 100 a day) and outside of it.
	api.createTransaction(map[string]any{"type": "expense", "account_id": checking.ID, "amount": 3000, "category_id": food.ID, "occurred_on": daysFromToday(-5)})
	api.createTransaction(map[string]any{"type": "expense", "account_id": checking.ID, "amount": 6000, "category_id": food.ID, "occurred_on": daysFromToday(-40)})

	overdue := api.createRecurring(map[string]any{"name": "Rent", "type": "expense", "account_id": checking.ID, "amount": 700, "interval_unit": "month", "start_on": daysFromToday(-3)})
	paidItem := api.createRecurring(map[string]any{"name": "Phone", "type": "expense", "account_id": checking.ID, "amount": 900, "interval_unit": "month", "start_on": daysFromToday(-2)})
	later := api.createRecurring(map[string]any{"name": "Gym", "type": "expense", "account_id": checking.ID, "amount": 5000, "interval_unit": "month", "start_on": daysFromToday(2)})
	api.createRecurring(map[string]any{"name": "Pay", "type": "income", "account_id": checking.ID, "amount": 2000, "interval_unit": "month", "start_on": daysFromToday(3)})
	api.createRecurring(map[string]any{"name": "Fund", "type": "expense", "account_id": savings.ID, "amount": 111, "interval_unit": "month", "start_on": daysFromToday(1)})
	api.do(http.MethodPost, "/api/v1/recurring/"+paidItem.ID.String()+"/payments", map[string]any{}).expect(http.StatusCreated)
	// A transaction already recorded for a future day.
	// End of today: 100000 - 3000 - 6000 - 900 (the paid Phone) = 90100.
	api.createTransaction(map[string]any{"type": "expense", "account_id": checking.ID, "amount": 1500, "occurred_on": daysFromToday(4)})

	s := api.dailySummary(rangeQuery(0, 5))
	c := s.Currencies[0]
	if len(c.Days) != 1 || c.Days[0].Balance != 90100 || c.Days[0].Expense != 900 {
		t.Fatalf("unexpected today: %+v", c.Days)
	}
	if c.ProjectionBasis.AverageDailyExpense != 100 {
		t.Fatalf("unexpected average: %d", c.ProjectionBasis.AverageDailyExpense)
	}
	if got, want := func() []int64 {
		out := []int64{}
		for _, p := range c.Projection {
			out = append(out, p.Balance)
		}
		return out
	}(), []int64{89300, 84200, 86100, 84500, 84400}; !slices.Equal(got, want) {
		t.Fatalf("projection %v, want %v", got, want)
	}
	if c.Projection[0].Date != daysFromToday(1) || c.Projection[4].Date != daysFromToday(5) {
		t.Fatalf("unexpected projection dates: %+v", c.Projection)
	}

	basis := c.ProjectionBasis.Recurring
	if len(basis) != 3 {
		t.Fatalf("expected 3 applied occurrences (paid and savings ones excluded): %+v", basis)
	}
	if basis[0].RecurringID != overdue.ID || basis[0].DueOn != daysFromToday(-3) || basis[0].AppliedOn != daysFromToday(1) || basis[0].Amount != -700 {
		t.Fatalf("unexpected overdue occurrence: %+v", basis[0])
	}
	if basis[1].RecurringID != later.ID || basis[1].DueOn != daysFromToday(2) || basis[1].Amount != -5000 || basis[2].Name != "Pay" || basis[2].Amount != 2000 {
		t.Fatalf("unexpected occurrences: %+v", basis)
	}
}

func TestDailySummaryOwnerFilter(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	mine := api.createOwnedAccount("Mine", "me")
	shared := api.createOwnedAccount("Joint", "shared")
	api.createTransaction(map[string]any{"type": "income", "account_id": mine.ID, "amount": 400, "occurred_on": daysFromToday(0)})
	api.createTransaction(map[string]any{"type": "income", "account_id": shared.ID, "amount": 70, "occurred_on": daysFromToday(0)})

	for owner, want := range map[string]int64{"": 470, "me": 400, "shared": 70} {
		s := api.dailySummary(rangeQuery(0, 0) + "&owner=" + owner)
		if got := s.Currencies[0].Days[0].Balance; got != want {
			t.Fatalf("owner %q: balance %d, want %d", owner, got, want)
		}
	}
	api.do(http.MethodGet, "/api/v1/summary/daily?owner=Ana", nil).expectError(http.StatusUnprocessableEntity)
}

func TestDailySummaryCurrenciesAndArchivedAccounts(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	eur := api.createAccount("Euros", "EUR", 100)
	usd := api.createAccount("Dollars", "USD", 9000)
	api.createAccount("Pesos", "MXN", 0)
	old := api.createAccount("Old", "GBP", 5)
	api.do(http.MethodPatch, "/api/v1/accounts/"+old.ID.String(), map[string]any{"archived": true}).expect(http.StatusOK)
	api.createTransaction(map[string]any{"type": "expense", "account_id": eur.ID, "amount": 30, "occurred_on": daysFromToday(0)})
	api.createTransaction(map[string]any{"type": "income", "account_id": usd.ID, "amount": 1000, "occurred_on": daysFromToday(0)})

	s := api.dailySummary(rangeQuery(-1, 0))
	if len(s.Currencies) != 3 || len(s.Accounts) != 3 {
		t.Fatalf("archived account must be excluded: %+v", s.Currencies)
	}
	got := map[string][]int64{}
	for _, c := range s.Currencies {
		got[c.Currency] = dayBalances(c.Days)
	}
	if !slices.Equal(got["EUR"], []int64{100, 70}) || !slices.Equal(got["USD"], []int64{9000, 10000}) || !slices.Equal(got["MXN"], []int64{0, 0}) {
		t.Fatalf("currencies mixed or wrong: %v", got)
	}
	if s.Currencies[0].Currency != "EUR" || s.Currencies[1].Currency != "MXN" || s.Currencies[2].Currency != "USD" {
		t.Fatalf("expected currencies sorted by code: %+v", s.Currencies)
	}
}

func TestDailySummaryValidation(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	for name, query := range map[string]string{
		"to before from": rangeQuery(0, -1),
		"span too long":  rangeQuery(-92, 0),
		"bad from":       "from=yesterday",
		"bad to":         "to=2026-13-45",
	} {
		api.do(http.MethodGet, "/api/v1/summary/daily?"+query, nil).expectError(http.StatusUnprocessableEntity)
		_ = name
	}
	api.do(http.MethodGet, "/api/v1/summary/daily?"+rangeQuery(-91, 0), nil).expect(http.StatusOK)
	api.as("").do(http.MethodGet, "/api/v1/summary/daily", nil).expectError(http.StatusUnauthorized)
}

func TestDailySummaryIsolatedPerWorkspace(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	mine := api.createAccount("Mine", "EUR", 100)
	other := api.newUser("other@example.com", "Other")
	theirs := other.createAccount("Theirs", "USD", 5000)
	other.createTransaction(map[string]any{"type": "income", "account_id": theirs.ID, "amount": 999, "occurred_on": daysFromToday(0)})
	api.createTransaction(map[string]any{"type": "income", "account_id": mine.ID, "amount": 1, "occurred_on": daysFromToday(0)})

	s := api.dailySummary(rangeQuery(0, 0))
	if len(s.Currencies) != 1 || s.Currencies[0].Currency != "EUR" || s.Currencies[0].Days[0].Balance != 101 || len(s.Accounts) != 1 {
		t.Fatalf("other workspace leaked: %+v", s)
	}
}
