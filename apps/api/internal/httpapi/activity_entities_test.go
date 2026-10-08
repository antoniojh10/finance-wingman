package httpapi

import (
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/finance"
)

func (a *testAPI) activityCount() int {
	a.t.Helper()
	return len(a.activity("?limit=200").Items)
}

func TestActivityRecordsAccountsCategoriesAndBudgets(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)

	account := api.createAccount("Secret savings", "MXN", 987654)
	food := api.createCategory("Secret food", "expense")
	id := account.ID.String()
	api.do(http.MethodPatch, "/api/v1/accounts/"+id, map[string]any{"name": "Secret rename", "initial_balance": 1}).expect(http.StatusOK)
	api.do(http.MethodPatch, "/api/v1/accounts/"+id, map[string]any{"archived": true}).expect(http.StatusOK)
	api.do(http.MethodPatch, "/api/v1/accounts/"+id, map[string]any{"archived": false}).expect(http.StatusOK)
	api.do(http.MethodPatch, "/api/v1/categories/"+food.ID.String(), map[string]any{"name": "Secret meals", "color": "#22c55e"}).expect(http.StatusOK)
	api.do(http.MethodPatch, "/api/v1/categories/"+food.ID.String(), map[string]any{"archived": true}).expect(http.StatusOK)
	api.setBudgets("2026-10", budgetItem(food, "MXN", 123456))
	api.setBudgets("2026-10", map[string]any{"category_id": food.ID, "currency": "MXN", "clear": true})
	api.do(http.MethodDelete, "/api/v1/categories/"+food.ID.String(), nil).expect(http.StatusNoContent)
	api.do(http.MethodDelete, "/api/v1/accounts/"+id, nil).expect(http.StatusNoContent)

	res := api.do(http.MethodGet, "/api/v1/activity?limit=200", nil).expect(http.StatusOK)
	// The actor's email is the only personal value the feed may carry.
	body := strings.ReplaceAll(string(res.Body), "owner@example.com", "")
	for _, secret := range []string{"Secret", "987654", "123456", "#22c55e"} {
		if strings.Contains(body, secret) {
			t.Fatalf("the log must hold metadata only, found %q in %s", secret, res.Body)
		}
	}
	var all finance.ActivityPage
	res.decode(&all)
	want := []string{
		"account.deleted", "category.deleted", "budget.cleared", "budget.set", "category.archived", "category.updated",
		"account.unarchived", "account.archived", "account.updated", "category.created", "account.created",
	}
	if got := actions(all); !slices.Equal(got, want) {
		t.Fatalf("expected %v (newest first), got %v", want, got)
	}

	byAction := map[string]finance.ActivityEntry{}
	for _, e := range all.Items {
		byAction[e.Action] = e
	}
	if e := byAction["account.updated"]; e.EntityType != "account" || e.EntityID != account.ID ||
		e.Details.Type != "checking" || e.Details.Currency != "MXN" || !slices.Equal(e.Details.Changed, []string{"name", "initial_balance"}) {
		t.Fatalf("account update: %+v", e)
	}
	if e := byAction["category.updated"]; e.EntityType != "category" || e.EntityID != food.ID ||
		e.Details.Type != "expense" || !slices.Equal(e.Details.Changed, []string{"name", "color"}) {
		t.Fatalf("category update: %+v", e)
	}
	for _, action := range []string{"budget.set", "budget.cleared"} {
		e := byAction[action]
		d := e.Details
		if e.EntityType != "budget" || e.EntityID != food.ID || d.CategoryID == nil || *d.CategoryID != food.ID ||
			d.Currency != "MXN" || d.Month != "2026-10" || d.Changed != nil {
			t.Fatalf("%s: %+v", action, e)
		}
	}

	// Filtering by kind of record and by record.
	for entityType, count := range map[string]int{"account": 5, "category": 4, "budget": 2, "transaction": 0, "recurring_item": 0} {
		if page := api.activity("?entity_type=" + entityType); len(page.Items) != count {
			t.Fatalf("entity_type=%s: expected %d entries, got %v", entityType, count, actions(page))
		}
	}
	if page := api.activity("?entity_id=" + id); len(page.Items) != 5 {
		t.Fatalf("entity_id of the account: %v", actions(page))
	}
}

func TestActivityIgnoresNoOpAndFailedEntityChanges(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	account := api.createAccount("Checking", "MXN", 100)
	food := api.createCategory("Food", "expense")
	item := api.weeklyItem(account, nil)
	api.setBudgets("2026-10", budgetItem(food, "MXN", 500))
	api.seedMonthly(account, "Netflix", nil, 1000, 1000, 1000)
	key := api.suggestions()[0].Key
	api.do(http.MethodPost, "/api/v1/recurring/suggestions/dismiss", map[string]any{"key": key}).expect(http.StatusNoContent)
	before := api.activityCount()

	id := account.ID.String()
	// Saving the same values changes nothing.
	api.do(http.MethodPatch, "/api/v1/accounts/"+id, map[string]any{"name": "Checking", "initial_balance": 100, "archived": false}).expect(http.StatusOK)
	api.do(http.MethodPatch, "/api/v1/categories/"+food.ID.String(), map[string]any{"name": "Food", "archived": false}).expect(http.StatusOK)
	api.do(http.MethodPatch, "/api/v1/recurring/"+item.ID.String(), map[string]any{"name": "Gym", "amount": 50000, "status": "active"}).expect(http.StatusOK)
	api.setBudgets("2026-10", budgetItem(food, "MXN", 500))
	api.do(http.MethodPost, "/api/v1/recurring/suggestions/dismiss", map[string]any{"key": key}).expect(http.StatusNoContent)

	// Failed writes leave nothing behind.
	api.do(http.MethodPost, "/api/v1/accounts", map[string]any{"name": "Checking", "type": "cash", "currency": "MXN"}).expect(http.StatusConflict)
	api.do(http.MethodPost, "/api/v1/categories", map[string]any{"name": "Food", "kind": "expense"}).expect(http.StatusConflict)
	api.do(http.MethodPatch, "/api/v1/accounts/"+missingID, map[string]any{"name": "x"}).expect(http.StatusNotFound)
	api.do(http.MethodDelete, "/api/v1/categories/"+missingID, nil).expect(http.StatusNotFound)
	api.do(http.MethodDelete, "/api/v1/accounts/"+missingID, nil).expect(http.StatusNotFound)
	api.do(http.MethodPatch, "/api/v1/recurring/"+missingID, map[string]any{"name": "x"}).expect(http.StatusNotFound)
	api.do(http.MethodPost, "/api/v1/recurring", map[string]any{
		"name": "Gym", "type": "expense", "account_id": account.ID, "amount": 1, "interval_unit": "week",
	}).expect(http.StatusConflict)
	api.do(http.MethodPut, "/api/v1/budgets/2026-10", map[string]any{"items": []map[string]any{
		budgetItem(food, "EUR", 1), {"category_id": missingID, "currency": "MXN", "amount": 1},
	}}).expect(http.StatusUnprocessableEntity)
	other := api.createAccount("Other", "MXN", 0)
	api.createTransaction(map[string]any{"type": "expense", "account_id": other.ID, "amount": 1})
	api.do(http.MethodDelete, "/api/v1/accounts/"+other.ID.String(), nil).expect(http.StatusConflict)

	// Only the creation of "Other" and of its transaction count.
	if added := api.activityCount() - before; added != 2 {
		t.Fatalf("expected 2 new entries, got %d: %v", added, actions(api.activity("?limit=200")))
	}
}

func TestActivityRecordsRecurringExportAndSuggestionsWithoutValues(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	account := api.createAccount("Card", "MXN", 0)
	item := api.weeklyItem(account, map[string]any{"name": "Secret gym", "notes": "secret notes"})
	api.do(http.MethodPatch, "/api/v1/recurring/"+item.ID.String(), map[string]any{
		"name": "Secret gym 2", "amount": 987123, "notes": "secret notes 2", "status": "paused",
	}).expect(http.StatusOK)
	api.seedMonthly(account, "Secret Netflix", nil, 1000, 1000, 1000)
	key := api.suggestions()[0].Key
	api.do(http.MethodPost, "/api/v1/recurring/suggestions/dismiss", map[string]any{"key": key}).expect(http.StatusNoContent)
	api.do(http.MethodGet, "/api/v1/export", nil).expect(http.StatusOK)

	res := api.do(http.MethodGet, "/api/v1/activity?limit=200", nil).expect(http.StatusOK)
	for _, secret := range []string{"Secret", "secret", "987123", "Card"} {
		if strings.Contains(string(res.Body), secret) {
			t.Fatalf("the log must hold metadata only, found %q in %s", secret, res.Body)
		}
	}

	recurring := api.activity("?entity_type=recurring_item")
	if got := actions(recurring); !slices.Equal(got, []string{"recurring_item.updated", "recurring_item.created"}) {
		t.Fatalf("recurring actions: %v", got)
	}
	if e := recurring.Items[0]; e.EntityID != item.ID || e.Details.Type != "expense" || e.Details.AccountID == nil || *e.Details.AccountID != account.ID ||
		!slices.Equal(e.Details.Changed, []string{"name", "amount", "notes", "status"}) {
		t.Fatalf("recurring update: %+v", e)
	}

	suggestion := api.activity("?entity_type=recurring_suggestion")
	if len(suggestion.Items) != 1 || suggestion.Items[0].Action != "recurring_suggestion.dismissed" ||
		suggestion.Items[0].EntityID != account.ID || suggestion.Items[0].Details.Type != "expense" {
		t.Fatalf("dismissed suggestion: %+v", suggestion)
	}

	export := api.activity("?entity_type=workspace")
	if len(export.Items) != 1 || export.Items[0].Action != "export.requested" || export.Items[0].EntityID != api.workspace.ID ||
		export.Items[0].Details.Format != "json" || export.Items[0].Actor == nil || export.Items[0].Actor.ID != api.owner.ID {
		t.Fatalf("export entry: %+v", export)
	}
	api.do(http.MethodGet, "/api/v1/export?format=csv", nil).expect(http.StatusOK)
	if page := api.activity("?entity_type=workspace"); len(page.Items) != 2 || page.Items[0].Details.Format != "csv" {
		t.Fatalf("a second export should be logged with its format: %+v", page)
	}
}
