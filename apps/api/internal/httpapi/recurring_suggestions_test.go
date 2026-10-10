package httpapi

import (
	"net/http"
	"testing"
	"time"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/finance"
)

// seedMonthly records one expense every 30 days ending 5 days ago, oldest
// first, so the series is regular and still active.
func (a *testAPI) seedMonthly(account finance.Account, description string, categoryID any, amounts ...int64) []finance.Transaction {
	a.t.Helper()
	var txs []finance.Transaction
	for i, amount := range amounts {
		body := map[string]any{
			"type": "expense", "account_id": account.ID, "amount": amount, "description": description,
			"occurred_on": daysFromToday(-5 - 30*(len(amounts)-1-i)),
		}
		if categoryID != nil {
			body["category_id"] = categoryID
		}
		txs = append(txs, a.createTransaction(body))
	}
	return txs
}

func (a *testAPI) suggestions() []finance.RecurringSuggestion {
	a.t.Helper()
	var list struct {
		Items []finance.RecurringSuggestion `json:"items"`
	}
	a.do(http.MethodGet, "/api/v1/recurring/suggestions", nil).expect(http.StatusOK).decode(&list)
	return list.Items
}

func TestListRecurringSuggestions(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	account := api.createAccount("Card", "MXN", 0)
	category := api.createCategory("Entertainment", "expense")
	txs := api.seedMonthly(account, "Spotify #4412", category.ID, 11500, 11900, 11900)
	api.seedMonthly(account, "Tacos", nil, 15000, 15000) // too few occurrences
	api.createTransaction(map[string]any{                // irregular one-off
		"type": "expense", "account_id": account.ID, "amount": 90000, "description": "Tacos", "occurred_on": daysFromToday(-47),
	})

	got := api.suggestions()
	if len(got) != 1 {
		t.Fatalf("expected one suggestion, got %+v", got)
	}
	s := got[0]
	if s.Name != "Spotify #4412" || s.Type != "expense" || s.AccountID != account.ID || s.Currency != "MXN" ||
		s.Amount != 11900 || s.Frequency != "monthly" || s.IntervalUnit != "month" || s.IntervalCount != 1 ||
		s.CategoryID == nil || *s.CategoryID != category.ID || s.CategoryName == nil || *s.CategoryName != "Entertainment" ||
		s.Key == "" || s.NextDueOn == nil || s.Confidence <= 0 || s.Confidence > 1 {
		t.Fatalf("unexpected suggestion: %+v", s)
	}
	if len(s.TransactionIDs) != 3 || s.TransactionIDs[0] != txs[0].ID || s.TransactionIDs[2] != txs[2].ID {
		t.Fatalf("expected the 3 matching transactions oldest first: %+v", s.TransactionIDs)
	}
}

func TestRecurringSuggestionsExcludeLinkedAndTransfers(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	account := api.createAccount("Card", "MXN", 0)
	other := api.createAccount("Savings", "MXN", 0)
	txs := api.seedMonthly(account, "Gym", nil, 50000, 50000, 50000)
	for i := 0; i < 3; i++ {
		api.createTransaction(map[string]any{
			"type": "transfer", "account_id": account.ID, "destination_account_id": other.ID, "amount": 1000,
			"description": "Savings", "occurred_on": daysFromToday(-5 - 30*i),
		})
	}
	got := api.suggestions()
	if len(got) != 1 || got[0].Name != "Gym" {
		t.Fatalf("expected only the gym suggestion (transfers excluded): %+v", got)
	}

	// Linking one of the transactions leaves two unlinked: too few.
	item := api.createRecurring(map[string]any{
		"name": "Gym", "type": "expense", "account_id": account.ID, "amount": 50000, "interval_unit": "month", "start_on": daysFromToday(-65),
	})
	api.do(http.MethodPut, "/api/v1/transactions/"+txs[0].ID.String()+"/recurring", map[string]any{"recurring_id": item.ID}).expect(http.StatusOK)
	if got := api.suggestions(); len(got) != 0 {
		t.Fatalf("linked transactions must not be suggested: %+v", got)
	}
}

func TestAcceptRecurringSuggestion(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	account := api.createAccount("Card", "MXN", 0)
	category := api.createCategory("Entertainment", "expense")
	txs := api.seedMonthly(account, "Netflix", category.ID, 18900, 18900, 19900)
	s := api.suggestions()[0]

	var item finance.RecurringItem
	api.do(http.MethodPost, "/api/v1/recurring/suggestions/accept", map[string]any{"key": s.Key}).
		expect(http.StatusCreated).decode(&item)
	if item.Name != "Netflix" || item.Type != "expense" || item.AccountID != account.ID || item.Amount != s.Amount ||
		item.IntervalUnit != "month" || item.IntervalCount != 1 || item.StartOn != s.StartOn || item.Status != "active" ||
		item.CategoryID == nil || *item.CategoryID != category.ID {
		t.Fatalf("unexpected item: %+v", item)
	}
	if item.LastPayment == nil || item.LastPayment.TransactionID != txs[2].ID {
		t.Fatalf("expected the latest match as last payment: %+v", item.LastPayment)
	}

	periods := map[string]bool{}
	for _, tx := range txs {
		var got finance.Transaction
		api.do(http.MethodGet, "/api/v1/transactions/"+tx.ID.String(), nil).expect(http.StatusOK).decode(&got)
		if got.RecurringID == nil || *got.RecurringID != item.ID || got.RecurringDueOn == nil {
			t.Fatalf("transaction should be linked: %+v", got)
		}
		periods[*got.RecurringDueOn] = true
	}
	if len(periods) != 3 {
		t.Fatalf("each transaction should settle its own period: %v", periods)
	}
	if got := api.suggestions(); len(got) != 0 {
		t.Fatalf("an accepted suggestion must disappear: %+v", got)
	}

	// The suggestion is gone, so accepting again finds nothing.
	api.do(http.MethodPost, "/api/v1/recurring/suggestions/accept", map[string]any{"key": s.Key}).expect(http.StatusNotFound)
}

func TestAcceptRecurringSuggestionOverrides(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	account := api.createAccount("Card", "MXN", 0)
	api.seedMonthly(account, "Netflix", nil, 18900, 18900, 18900)
	s := api.suggestions()[0]

	var item finance.RecurringItem
	api.do(http.MethodPost, "/api/v1/recurring/suggestions/accept", map[string]any{"key": s.Key, "name": "Netflix Premium", "amount": 25000}).
		expect(http.StatusCreated).decode(&item)
	if item.Name != "Netflix Premium" || item.Amount != 25000 {
		t.Fatalf("overrides not applied: %+v", item)
	}

	// The category override is applied in the same transaction; a category
	// of the wrong kind rolls the accept back.
	api.seedMonthly(account, "Spotify", nil, 9900, 9900, 9900)
	var spotify string
	for _, sg := range api.suggestions() {
		if sg.Name == "Spotify" {
			spotify = sg.Key
		}
	}
	salary := api.createCategory("Salary", "income")
	api.do(http.MethodPost, "/api/v1/recurring/suggestions/accept", map[string]any{"key": spotify, "category_id": salary.ID}).
		expect(http.StatusUnprocessableEntity)
	music := api.createCategory("Music", "expense")
	api.do(http.MethodPost, "/api/v1/recurring/suggestions/accept", map[string]any{"key": spotify, "category_id": music.ID}).
		expect(http.StatusCreated).decode(&item)
	if item.CategoryID == nil || *item.CategoryID != music.ID {
		t.Fatalf("category override not applied: %+v", item)
	}
}

func TestAcceptRecurringSuggestionErrors(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	account := api.createAccount("Card", "MXN", 0)
	api.seedMonthly(account, "Netflix", nil, 18900, 18900, 18900)
	s := api.suggestions()[0]

	api.do(http.MethodPost, "/api/v1/recurring/suggestions/accept", map[string]any{"key": "not a key"}).expect(http.StatusUnprocessableEntity)
	api.do(http.MethodPost, "/api/v1/recurring/suggestions/accept", map[string]any{"key": ""}).expect(http.StatusUnprocessableEntity)
	api.do(http.MethodPost, "/api/v1/recurring/suggestions/accept", map[string]any{"key": s.Key, "amount": 0}).expect(http.StatusUnprocessableEntity)

	// A well-formed key whose pattern is not detected.
	dismissAndMissing := api.suggestions()[0]
	api.do(http.MethodPost, "/api/v1/recurring/suggestions/dismiss", map[string]any{"key": dismissAndMissing.Key}).expect(http.StatusNoContent)
	api.do(http.MethodPost, "/api/v1/recurring/suggestions/accept", map[string]any{"key": dismissAndMissing.Key}).expect(http.StatusNotFound)

	// Nothing was created or linked by the failed attempts.
	var list struct{ Items []finance.RecurringItem }
	api.do(http.MethodGet, "/api/v1/recurring", nil).expect(http.StatusOK).decode(&list)
	if len(list.Items) != 0 {
		t.Fatalf("no item should exist: %+v", list.Items)
	}
}

func TestAcceptRecurringSuggestionNameClash(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	account := api.createAccount("Card", "MXN", 0)
	txs := api.seedMonthly(account, "Netflix", nil, 18900, 18900, 18900)
	existing := api.createRecurring(map[string]any{
		"name": "netflix", "type": "expense", "account_id": account.ID, "amount": 500, "interval_unit": "month",
	})
	s := api.suggestions()[0]

	api.do(http.MethodPost, "/api/v1/recurring/suggestions/accept", map[string]any{"key": s.Key}).expect(http.StatusConflict)

	var list struct{ Items []finance.RecurringItem }
	api.do(http.MethodGet, "/api/v1/recurring", nil).expect(http.StatusOK).decode(&list)
	if len(list.Items) != 1 || list.Items[0].ID != existing.ID {
		t.Fatalf("no item should have been created: %+v", list.Items)
	}
	for _, tx := range txs {
		var got finance.Transaction
		api.do(http.MethodGet, "/api/v1/transactions/"+tx.ID.String(), nil).expect(http.StatusOK).decode(&got)
		if got.RecurringID != nil || got.RecurringDueOn != nil {
			t.Fatalf("no transaction should have been linked: %+v", got)
		}
	}
	if got := api.suggestions(); len(got) != 1 {
		t.Fatalf("the suggestion should remain available: %+v", got)
	}

	// Renaming resolves the clash.
	api.do(http.MethodPost, "/api/v1/recurring/suggestions/accept", map[string]any{"key": s.Key, "name": "Netflix HD"}).expect(http.StatusCreated)
}

func TestDismissRecurringSuggestion(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	account := api.createAccount("Card", "MXN", 0)
	api.seedMonthly(account, "Netflix", nil, 18900, 18900, 18900)
	api.seedMonthly(account, "Spotify", nil, 11900, 11900, 11900)
	got := api.suggestions()
	if len(got) != 2 {
		t.Fatalf("expected 2 suggestions: %+v", got)
	}
	target := got[0]

	// Dismissing twice is fine.
	api.do(http.MethodPost, "/api/v1/recurring/suggestions/dismiss", map[string]any{"key": target.Key}).expect(http.StatusNoContent)
	api.do(http.MethodPost, "/api/v1/recurring/suggestions/dismiss", map[string]any{"key": target.Key}).expect(http.StatusNoContent)

	left := api.suggestions()
	if len(left) != 1 || left[0].Key == target.Key {
		t.Fatalf("the dismissed suggestion must not reappear: %+v", left)
	}

	// A new matching transaction does not bring it back either.
	api.createTransaction(map[string]any{
		"type": "expense", "account_id": account.ID, "amount": 11900, "description": target.Name, "occurred_on": daysFromToday(-1),
	})
	for _, s := range api.suggestions() {
		if s.Key == target.Key {
			t.Fatalf("dismissed suggestion reappeared: %+v", s)
		}
	}

	api.do(http.MethodPost, "/api/v1/recurring/suggestions/dismiss", map[string]any{"key": "garbage"}).expect(http.StatusUnprocessableEntity)
}

func TestRecurringSuggestionsRequireAuth(t *testing.T) {
	t.Parallel()
	anon := newTestAPI(t).as("")
	anon.do(http.MethodGet, "/api/v1/recurring/suggestions", nil).expect(http.StatusUnauthorized)
	anon.do(http.MethodPost, "/api/v1/recurring/suggestions/accept", map[string]any{"key": "x"}).expect(http.StatusUnauthorized)
	anon.do(http.MethodPost, "/api/v1/recurring/suggestions/dismiss", map[string]any{"key": "x"}).expect(http.StatusUnauthorized)
	anon.do(http.MethodGet, "/api/v1/transactions/"+missingID+"/recurring-match", nil).expect(http.StatusUnauthorized)
}

func (a *testAPI) recurringMatch(txID string) finance.RecurringMatch {
	a.t.Helper()
	var m finance.RecurringMatch
	a.do(http.MethodGet, "/api/v1/transactions/"+txID+"/recurring-match", nil).expect(http.StatusOK).decode(&m)
	return m
}

func TestMatchRecurringItem(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	account := api.createAccount("Card", "MXN", 0)
	other := api.createAccount("Other", "MXN", 0)
	// Monthly items starting on the 1st two months ago, paid on their second
	// due date: day 1 never gets clamped by short months.
	now := time.Now().UTC()
	firstDue := time.Date(now.Year(), now.Month()-2, 1, 0, 0, 0, 0, time.UTC)
	startOn := firstDue.Format(time.DateOnly)
	paidOn := firstDue.AddDate(0, 1, 0).Format(time.DateOnly)
	netflix := api.createRecurring(map[string]any{
		"name": "Netflix", "type": "expense", "account_id": account.ID, "amount": 18900, "interval_unit": "month", "start_on": startOn,
	})
	api.createRecurring(map[string]any{
		"name": "Netflix Premium", "type": "expense", "account_id": account.ID, "amount": 29900, "interval_unit": "month", "start_on": startOn,
	})
	api.createRecurring(map[string]any{
		"name": "Netflix Family", "type": "expense", "account_id": other.ID, "amount": 100, "interval_unit": "month", "start_on": startOn,
	})
	pay := func(desc string, amount int64, account finance.Account) finance.Transaction {
		return api.createTransaction(map[string]any{
			"type": "expense", "account_id": account.ID, "amount": amount, "description": desc, "occurred_on": paidOn,
		})
	}

	tx := pay("NETFLIX.COM 8812", 19500, account)
	m := api.recurringMatch(tx.ID.String())
	if m.Item == nil || m.Item.ID != netflix.ID || m.PeriodDueOn == nil || *m.PeriodDueOn != paidOn {
		t.Fatalf("expected the Netflix item and its closest period: %+v %v", m.Item, m.PeriodDueOn)
	}

	// The amount picks between two items whose names both match.
	if m := api.recurringMatch(pay("Netflix", 29500, account).ID.String()); m.Item == nil || m.Item.Name != "Netflix Premium" {
		t.Fatalf("expected Netflix Premium by amount: %+v", m.Item)
	}

	// No match: unrelated description, no description, other account items only.
	for _, tc := range []finance.Transaction{pay("Groceries", 19500, account), pay("", 19500, account)} {
		if m := api.recurringMatch(tc.ID.String()); m.Item != nil || m.PeriodDueOn != nil {
			t.Fatalf("expected no match for %q: %+v", tc.Description, m.Item)
		}
	}

	// Linked transactions are not matched again.
	api.do(http.MethodPut, "/api/v1/transactions/"+tx.ID.String()+"/recurring", map[string]any{"recurring_id": netflix.ID}).expect(http.StatusOK)
	if m := api.recurringMatch(tx.ID.String()); m.Item != nil {
		t.Fatalf("a linked transaction should not match: %+v", m.Item)
	}

	// Paused items are not candidates.
	api.do(http.MethodPatch, "/api/v1/recurring/"+netflix.ID.String(), map[string]any{"status": "paused"}).expect(http.StatusOK)
	if m := api.recurringMatch(pay("Netflix", 18900, account).ID.String()); m.Item == nil || m.Item.Name != "Netflix Premium" {
		t.Fatalf("expected the only active match: %+v", m.Item)
	}

	api.do(http.MethodGet, "/api/v1/transactions/"+missingID+"/recurring-match", nil).expect(http.StatusNotFound)
}
