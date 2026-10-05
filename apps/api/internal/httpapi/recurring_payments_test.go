package httpapi

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/finance"
)

func daysFromToday(n int) string {
	return time.Now().UTC().AddDate(0, 0, n).Format(time.DateOnly)
}

// weeklyItem creates a weekly expense whose due dates are today-10,
// today-3 and today+4 (and so on): the current period is overdue.
func (a *testAPI) weeklyItem(account finance.Account, body map[string]any) finance.RecurringItem {
	a.t.Helper()
	req := map[string]any{
		"name": "Gym", "type": "expense", "account_id": account.ID, "amount": 50000,
		"interval_unit": "week", "start_on": daysFromToday(-10),
	}
	for k, v := range body {
		req[k] = v
	}
	return a.createRecurring(req)
}

func (a *testAPI) getRecurring(id string) finance.RecurringItem {
	a.t.Helper()
	var item finance.RecurringItem
	a.do(http.MethodGet, "/api/v1/recurring/"+id, nil).expect(http.StatusOK).decode(&item)
	return item
}

func TestRegisterRecurringPayment(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	account := api.createAccount("Card", "MXN", 0)
	category := api.createCategory("Health", "expense")
	item := api.weeklyItem(account, map[string]any{"category_id": category.ID})

	if item.CurrentPeriod == nil || item.CurrentPeriod.DueOn != daysFromToday(-3) || item.CurrentPeriod.Status != "overdue" || item.LastPayment != nil {
		t.Fatalf("expected an overdue current period: %+v", item.CurrentPeriod)
	}

	var tx finance.Transaction
	api.do(http.MethodPost, "/api/v1/recurring/"+item.ID.String()+"/payments", map[string]any{}).expect(http.StatusCreated).decode(&tx)
	if tx.Type != "expense" || tx.AccountID != account.ID || tx.Amount != 50000 || tx.Description != "Gym" ||
		tx.CategoryName == nil || *tx.CategoryName != "Health" || tx.OccurredOn != daysFromToday(0) {
		t.Fatalf("unexpected payment: %+v", tx)
	}
	if tx.RecurringID == nil || *tx.RecurringID != item.ID || tx.RecurringDueOn == nil || *tx.RecurringDueOn != daysFromToday(-3) {
		t.Fatalf("payment should link to the closest period: %+v", tx)
	}

	got := api.getRecurring(item.ID.String())
	if got.CurrentPeriod.Status != "paid" || got.CurrentPeriod.DueOn != daysFromToday(-3) {
		t.Fatalf("expected paid period: %+v", got.CurrentPeriod)
	}
	if got.LastPayment == nil || got.LastPayment.TransactionID != tx.ID || got.LastPayment.Amount != 50000 || got.LastPayment.DueOn != daysFromToday(-3) {
		t.Fatalf("unexpected last payment: %+v", got.LastPayment)
	}

	// The list includes the same enrichment and the transactions payload
	// exposes recurring_id.
	var list struct{ Items []finance.RecurringItem }
	api.do(http.MethodGet, "/api/v1/recurring", nil).expect(http.StatusOK).decode(&list)
	if len(list.Items) != 1 || list.Items[0].CurrentPeriod.Status != "paid" || list.Items[0].LastPayment == nil {
		t.Fatalf("unexpected list: %+v", list.Items)
	}
	var fetched finance.Transaction
	api.do(http.MethodGet, "/api/v1/transactions/"+tx.ID.String(), nil).expect(http.StatusOK).decode(&fetched)
	if fetched.RecurringID == nil || *fetched.RecurringID != item.ID {
		t.Fatalf("transaction should expose recurring_id: %+v", fetched)
	}
}

func TestRegisterRecurringPaymentOverrides(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	account := api.createAccount("Card", "MXN", 0)
	item := api.weeklyItem(account, nil)

	// Past period, other date and a different amount; several payments can
	// settle the same period and the estimate never changes.
	for range 2 {
		var tx finance.Transaction
		api.do(http.MethodPost, "/api/v1/recurring/"+item.ID.String()+"/payments", map[string]any{
			"period": daysFromToday(-10), "date": daysFromToday(-9), "amount": 61234,
		}).expect(http.StatusCreated).decode(&tx)
		if tx.Amount != 61234 || tx.OccurredOn != daysFromToday(-9) || *tx.RecurringDueOn != daysFromToday(-10) {
			t.Fatalf("unexpected payment: %+v", tx)
		}
	}
	got := api.getRecurring(item.ID.String())
	if got.Amount != 50000 {
		t.Fatalf("estimate should not change, got %d", got.Amount)
	}
	if got.CurrentPeriod.Status != "overdue" {
		t.Fatalf("paying an old period must not settle the current one: %+v", got.CurrentPeriod)
	}

	path := "/api/v1/recurring/" + item.ID.String() + "/payments"
	api.do(http.MethodPost, path, map[string]any{"period": daysFromToday(-9)}).expectError(http.StatusUnprocessableEntity)
	api.do(http.MethodPost, path, map[string]any{"date": "nope"}).expectError(http.StatusUnprocessableEntity)
	api.do(http.MethodPost, path, map[string]any{"amount": 0}).expectError(http.StatusUnprocessableEntity)
	api.do(http.MethodPost, "/api/v1/recurring/"+missingID+"/payments", map[string]any{}).expectError(http.StatusNotFound)
	api.as("").do(http.MethodPost, path, map[string]any{}).expect(http.StatusUnauthorized)
}

func TestRegisterRecurringPaymentRules(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	account := api.createAccount("Card", "MXN", 0)

	for _, status := range []string{"paused", "cancelled"} {
		item := api.weeklyItem(account, map[string]any{"name": "Gym " + status})
		api.do(http.MethodPatch, "/api/v1/recurring/"+item.ID.String(), map[string]any{"status": status}).expect(http.StatusOK)
		api.do(http.MethodPost, "/api/v1/recurring/"+item.ID.String()+"/payments", map[string]any{}).expectError(http.StatusConflict)
		if got := api.getRecurring(item.ID.String()); got.CurrentPeriod != nil {
			t.Fatalf("%s item should have no current period: %+v", status, got.CurrentPeriod)
		}
	}

	archivedItem := api.weeklyItem(account, map[string]any{"name": "Archived account"})
	api.do(http.MethodPatch, "/api/v1/accounts/"+account.ID.String(), map[string]any{"archived": true}).expect(http.StatusOK)
	body := api.do(http.MethodPost, "/api/v1/recurring/"+archivedItem.ID.String()+"/payments", map[string]any{}).expectError(http.StatusUnprocessableEntity)
	if body.Errors[0].Location != "account_id" {
		t.Fatalf("expected an account_id error: %+v", body)
	}

	// Nothing is created when the payment fails.
	var page finance.TransactionPage
	api.do(http.MethodGet, "/api/v1/transactions", nil).expect(http.StatusOK).decode(&page)
	if page.Total != 0 {
		t.Fatalf("failed payments must not leave transactions: %+v", page)
	}
}

func TestRegisterRecurringPaymentInstallments(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	account := api.createAccount("Card", "MXN", 0)
	item := api.weeklyItem(account, map[string]any{"start_on": daysFromToday(-30), "total_payments": 2})

	// Due dates are today-30 and today-23: nothing is due after that.
	api.do(http.MethodPost, "/api/v1/recurring/"+item.ID.String()+"/payments", map[string]any{"period": daysFromToday(-16)}).expectError(http.StatusUnprocessableEntity)
	var tx finance.Transaction
	api.do(http.MethodPost, "/api/v1/recurring/"+item.ID.String()+"/payments", map[string]any{}).expect(http.StatusCreated).decode(&tx)
	if *tx.RecurringDueOn != daysFromToday(-23) {
		t.Fatalf("an exhausted schedule links to its last due date: %+v", tx)
	}
	got := api.getRecurring(item.ID.String())
	if got.CurrentPeriod.Status != "paid" || got.CurrentPeriod.DueOn != daysFromToday(-23) || got.NextDueOn != nil {
		t.Fatalf("unexpected exhausted item: %+v %+v", got.CurrentPeriod, got.NextDueOn)
	}
}

func TestLinkTransactionToRecurring(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	card := api.createAccount("Card", "MXN", 0)
	other := api.createAccount("Cash", "MXN", 0)
	food := api.createCategory("Food", "expense")
	salary := api.createCategory("Salary", "income")
	item := api.weeklyItem(card, nil)
	second := api.weeklyItem(card, map[string]any{"name": "Pool"})

	tx := api.createTransaction(map[string]any{
		"type": "expense", "account_id": card.ID, "amount": 1000, "category_id": food.ID, "occurred_on": daysFromToday(-9),
	})
	path := "/api/v1/transactions/" + tx.ID.String() + "/recurring"

	var linked finance.Transaction
	api.do(http.MethodPut, path, map[string]any{"recurring_id": item.ID}).expect(http.StatusOK).decode(&linked)
	if linked.RecurringID == nil || *linked.RecurringID != item.ID || *linked.RecurringDueOn != daysFromToday(-10) {
		t.Fatalf("should default to the closest period: %+v", linked)
	}

	// Already linked: the link is replaced.
	api.do(http.MethodPut, path, map[string]any{"recurring_id": second.ID, "period": daysFromToday(-3)}).expect(http.StatusOK).decode(&linked)
	if *linked.RecurringID != second.ID || *linked.RecurringDueOn != daysFromToday(-3) {
		t.Fatalf("expected the link to be replaced: %+v", linked)
	}

	// While linked, the type and account cannot change.
	api.do(http.MethodPut, "/api/v1/transactions/"+tx.ID.String(), map[string]any{
		"type": "expense", "account_id": other.ID, "amount": 1000,
	}).expectError(http.StatusUnprocessableEntity)
	api.do(http.MethodPut, "/api/v1/transactions/"+tx.ID.String(), map[string]any{
		"type": "expense", "account_id": card.ID, "amount": 2000, "category_id": food.ID, "occurred_on": daysFromToday(-9),
	}).expect(http.StatusOK).decode(&linked)
	if linked.RecurringID == nil || linked.Amount != 2000 {
		t.Fatalf("editing other fields keeps the link: %+v", linked)
	}

	// Unlink, twice (idempotent).
	for range 2 {
		api.do(http.MethodDelete, path, nil).expect(http.StatusOK).decode(&linked)
		if linked.RecurringID != nil || linked.RecurringDueOn != nil {
			t.Fatalf("expected an unlinked transaction: %+v", linked)
		}
	}

	// Rejections.
	otherAccount := api.createTransaction(map[string]any{"type": "expense", "account_id": other.ID, "amount": 1000})
	income := api.createTransaction(map[string]any{"type": "income", "account_id": card.ID, "amount": 1000, "category_id": salary.ID})
	transfer := api.createTransaction(map[string]any{"type": "transfer", "account_id": card.ID, "amount": 1000, "destination_account_id": other.ID})
	for _, bad := range []finance.Transaction{otherAccount, income, transfer} {
		body := api.do(http.MethodPut, "/api/v1/transactions/"+bad.ID.String()+"/recurring", map[string]any{"recurring_id": item.ID}).expectError(http.StatusUnprocessableEntity)
		if body.Errors[0].Location != "recurring_id" {
			t.Fatalf("expected a recurring_id error: %+v", body)
		}
	}
	api.do(http.MethodPut, path, map[string]any{"recurring_id": item.ID, "period": daysFromToday(-9)}).expectError(http.StatusUnprocessableEntity)
	api.do(http.MethodPut, path, map[string]any{"recurring_id": missingID}).expectError(http.StatusUnprocessableEntity)
	api.do(http.MethodPut, "/api/v1/transactions/"+missingID+"/recurring", map[string]any{"recurring_id": item.ID}).expectError(http.StatusNotFound)
	api.do(http.MethodDelete, "/api/v1/transactions/"+missingID+"/recurring", nil).expectError(http.StatusNotFound)
	api.as("").do(http.MethodPut, path, map[string]any{"recurring_id": item.ID}).expect(http.StatusUnauthorized)
	api.as("").do(http.MethodDelete, path, nil).expect(http.StatusUnauthorized)
}

func TestUpcomingRecurring(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	account := api.createAccount("Card", "MXN", 0)

	overdue := api.weeklyItem(account, map[string]any{"name": "Overdue"})                                               // due -3 (unpaid), +4
	paid := api.weeklyItem(account, map[string]any{"name": "Paid", "start_on": daysFromToday(-2), "interval_count": 2}) // due -2, +12
	api.do(http.MethodPost, "/api/v1/recurring/"+paid.ID.String()+"/payments", map[string]any{}).expect(http.StatusCreated)
	future := api.weeklyItem(account, map[string]any{"name": "Future", "start_on": daysFromToday(5)})
	paused := api.weeklyItem(account, map[string]any{"name": "Paused"})
	api.do(http.MethodPatch, "/api/v1/recurring/"+paused.ID.String(), map[string]any{"status": "paused"}).expect(http.StatusOK)

	type row struct {
		Item   finance.RecurringItem `json:"item"`
		DueOn  string                `json:"due_on"`
		Status string                `json:"status"`
	}
	var list struct{ Items []row }
	api.do(http.MethodGet, "/api/v1/recurring/upcoming?days=7", nil).expect(http.StatusOK).decode(&list)

	want := []struct{ name, due, status string }{
		{"Overdue", daysFromToday(-3), "overdue"},
		{"Overdue", daysFromToday(4), "pending"},
		{"Future", daysFromToday(5), "pending"},
	}
	if len(list.Items) != len(want) {
		t.Fatalf("expected %d rows, got %+v", len(want), list.Items)
	}
	for i, w := range want {
		got := list.Items[i]
		if got.Item.Name != w.name || got.DueOn != w.due || got.Status != w.status {
			t.Fatalf("row %d: got %s %s %s, want %+v", i, got.Item.Name, got.DueOn, got.Status, w)
		}
	}
	_ = overdue
	_ = future

	// Default window is 30 days and includes paid rows.
	api.do(http.MethodGet, "/api/v1/recurring/upcoming", nil).expect(http.StatusOK).decode(&list)
	foundPaid := false
	for _, r := range list.Items {
		if r.Item.Name == "Paid" && r.DueOn == daysFromToday(-2) {
			t.Fatalf("a paid current period in the past is not upcoming: %+v", r)
		}
		if r.Item.Name == "Paid" && r.DueOn == daysFromToday(12) && r.Status == "pending" {
			foundPaid = true
		}
	}
	if !foundPaid {
		t.Fatalf("expected the next Paid due date: %+v", list.Items)
	}

	api.do(http.MethodGet, "/api/v1/recurring/upcoming?days=0", nil).expectError(http.StatusUnprocessableEntity)
	api.do(http.MethodGet, "/api/v1/recurring/upcoming?days=400", nil).expectError(http.StatusUnprocessableEntity)
	api.as("").do(http.MethodGet, "/api/v1/recurring/upcoming", nil).expect(http.StatusUnauthorized)
}

func TestUpcomingRecurringShowsPaidDueToday(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	account := api.createAccount("Card", "MXN", 0)
	item := api.weeklyItem(account, map[string]any{"start_on": daysFromToday(0)})
	api.do(http.MethodPost, "/api/v1/recurring/"+item.ID.String()+"/payments", map[string]any{}).expect(http.StatusCreated)

	var list struct {
		Items []struct {
			DueOn  string `json:"due_on"`
			Status string `json:"status"`
		}
	}
	api.do(http.MethodGet, "/api/v1/recurring/upcoming?days=1", nil).expect(http.StatusOK).decode(&list)
	if len(list.Items) != 1 || list.Items[0].DueOn != daysFromToday(0) || list.Items[0].Status != "paid" {
		t.Fatalf("expected today's paid row: %+v", list.Items)
	}
}

func TestDeletingRecurringItemKeepsTransactions(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	account := api.createAccount("Card", "MXN", 0)
	item := api.weeklyItem(account, nil)
	var tx finance.Transaction
	api.do(http.MethodPost, "/api/v1/recurring/"+item.ID.String()+"/payments", map[string]any{}).expect(http.StatusCreated).decode(&tx)

	if _, err := api.pool.Exec(context.Background(), "DELETE FROM recurring_items WHERE id = $1", item.ID); err != nil {
		t.Fatal(err)
	}
	var got finance.Transaction
	api.do(http.MethodGet, "/api/v1/transactions/"+tx.ID.String(), nil).expect(http.StatusOK).decode(&got)
	if got.RecurringID != nil || got.RecurringDueOn != nil {
		t.Fatalf("expected the link to be cleared: %+v", got)
	}
}
