package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/auth"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/finance"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/oauth"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/workspace"
)

// financeTags are the OpenAPI tags of the operations that change finance
// data. Auth and workspace operations are logged by their own tickets.
var financeTags = []string{"Accounts", "Budgets", "Categories", "Recurring", "Transactions"}

// tracedReads are read operations that still leave a trace in the log.
var tracedReads = []string{"export-data"}

// mutation is one way of changing finance data over REST: prepare sets up
// what the request needs and returns the request together with the actions
// it must log.
type mutation struct {
	operation string
	prepare   func(a *testAPI) (do func(), actions []string)
}

// financeMutations lists every REST operation that changes finance data.
// TestEveryFinanceMutationIsCovered fails when the API has one that is not
// listed, and TestFinanceMutationsRecordActivity fails when one records
// nothing, so the log cannot silently miss a mutation.
func financeMutations() []mutation {
	post := func(a *testAPI, path string, body any, status int) func() {
		return func() { a.do(http.MethodPost, path, body).expect(status) }
	}
	return []mutation{
		{"create-account", func(a *testAPI) (func(), []string) {
			return post(a, "/api/v1/accounts", map[string]any{"name": "New", "type": "cash", "currency": "MXN"}, http.StatusCreated),
				[]string{finance.ActionAccountCreated}
		}},
		{"create-accounts-batch", func(a *testAPI) (func(), []string) {
			return post(a, "/api/v1/accounts/batch", map[string]any{"items": []map[string]any{
					{"name": "One", "type": "cash", "currency": "MXN"}, {"name": "Two", "type": "cash", "currency": "EUR"},
				}}, http.StatusCreated),
				[]string{finance.ActionAccountCreated, finance.ActionAccountCreated}
		}},
		{"update-account", func(a *testAPI) (func(), []string) {
			account := a.createAccount("Checking", "MXN", 0)
			return func() {
				a.do(http.MethodPatch, "/api/v1/accounts/"+account.ID.String(), map[string]any{"name": "Renamed"}).expect(http.StatusOK)
			}, []string{finance.ActionAccountUpdated}
		}},
		{"delete-account", func(a *testAPI) (func(), []string) {
			account := a.createAccount("Checking", "MXN", 0)
			return func() {
				a.do(http.MethodDelete, "/api/v1/accounts/"+account.ID.String(), nil).expect(http.StatusNoContent)
			}, []string{finance.ActionAccountDeleted}
		}},
		{"create-category", func(a *testAPI) (func(), []string) {
			return post(a, "/api/v1/categories", map[string]any{"name": "Food", "kind": "expense"}, http.StatusCreated),
				[]string{finance.ActionCategoryCreated}
		}},
		{"create-categories-batch", func(a *testAPI) (func(), []string) {
			return post(a, "/api/v1/categories/batch", map[string]any{"items": []map[string]any{
					{"name": "Food", "kind": "expense"}, {"name": "Pay", "kind": "income"},
				}}, http.StatusCreated),
				[]string{finance.ActionCategoryCreated, finance.ActionCategoryCreated}
		}},
		{"update-category", func(a *testAPI) (func(), []string) {
			category := a.createCategory("Food", "expense")
			return func() {
				a.do(http.MethodPatch, "/api/v1/categories/"+category.ID.String(), map[string]any{"color": "#22c55e"}).expect(http.StatusOK)
			}, []string{finance.ActionCategoryUpdated}
		}},
		{"delete-category", func(a *testAPI) (func(), []string) {
			category := a.createCategory("Food", "expense")
			return func() {
				a.do(http.MethodDelete, "/api/v1/categories/"+category.ID.String(), nil).expect(http.StatusNoContent)
			}, []string{finance.ActionCategoryDeleted}
		}},
		{"set-budgets", func(a *testAPI) (func(), []string) {
			category := a.createCategory("Food", "expense")
			return func() {
				a.do(http.MethodPut, "/api/v1/budgets/2026-10", map[string]any{"items": []map[string]any{
					budgetItem(category, "MXN", 1000), {"category_id": category.ID, "currency": "EUR", "clear": true},
				}}).expect(http.StatusOK)
			}, []string{finance.ActionBudgetSet, finance.ActionBudgetCleared}
		}},
		{"create-transaction", func(a *testAPI) (func(), []string) {
			account := a.createAccount("Checking", "MXN", 0)
			return post(a, "/api/v1/transactions", map[string]any{"type": "expense", "account_id": account.ID, "amount": 5}, http.StatusCreated),
				[]string{finance.ActionTransactionCreated}
		}},
		{"create-transactions-batch", func(a *testAPI) (func(), []string) {
			account := a.createAccount("Checking", "MXN", 0)
			item := map[string]any{"type": "expense", "account_id": account.ID, "amount": 5}
			return post(a, "/api/v1/transactions/batch", map[string]any{"items": []map[string]any{item, item}}, http.StatusCreated),
				[]string{finance.ActionTransactionCreated, finance.ActionTransactionCreated}
		}},
		{"replace-transaction", func(a *testAPI) (func(), []string) {
			account := a.createAccount("Checking", "MXN", 0)
			tx := a.createTransaction(map[string]any{"type": "expense", "account_id": account.ID, "amount": 5})
			return func() {
				a.do(http.MethodPut, "/api/v1/transactions/"+tx.ID.String(), map[string]any{"type": "expense", "account_id": account.ID, "amount": 6}).expect(http.StatusOK)
			}, []string{finance.ActionTransactionUpdated}
		}},
		{"delete-transaction", func(a *testAPI) (func(), []string) {
			account := a.createAccount("Checking", "MXN", 0)
			tx := a.createTransaction(map[string]any{"type": "expense", "account_id": account.ID, "amount": 5})
			return func() {
				a.do(http.MethodDelete, "/api/v1/transactions/"+tx.ID.String(), nil).expect(http.StatusNoContent)
			}, []string{finance.ActionTransactionDeleted}
		}},
		{"create-recurring-item", func(a *testAPI) (func(), []string) {
			account := a.createAccount("Card", "MXN", 0)
			return post(a, "/api/v1/recurring", map[string]any{
					"name": "Gym", "type": "expense", "account_id": account.ID, "amount": 500, "interval_unit": "month",
				}, http.StatusCreated),
				[]string{finance.ActionRecurringCreated}
		}},
		{"update-recurring-item", func(a *testAPI) (func(), []string) {
			item := a.weeklyItem(a.createAccount("Card", "MXN", 0), nil)
			return func() {
				a.do(http.MethodPatch, "/api/v1/recurring/"+item.ID.String(), map[string]any{"status": "paused"}).expect(http.StatusOK)
			}, []string{finance.ActionRecurringUpdated}
		}},
		{"register-recurring-payment", func(a *testAPI) (func(), []string) {
			item := a.weeklyItem(a.createAccount("Card", "MXN", 0), nil)
			return post(a, "/api/v1/recurring/"+item.ID.String()+"/payments", map[string]any{}, http.StatusCreated),
				[]string{finance.ActionTransactionCreated}
		}},
		{"link-transaction-recurring", func(a *testAPI) (func(), []string) {
			account := a.createAccount("Card", "MXN", 0)
			item := a.weeklyItem(account, nil)
			tx := a.createTransaction(map[string]any{"type": "expense", "account_id": account.ID, "amount": 5, "occurred_on": daysFromToday(-9)})
			return func() {
				a.do(http.MethodPut, "/api/v1/transactions/"+tx.ID.String()+"/recurring", map[string]any{"recurring_id": item.ID}).expect(http.StatusOK)
			}, []string{finance.ActionTransactionUpdated}
		}},
		{"unlink-transaction-recurring", func(a *testAPI) (func(), []string) {
			account := a.createAccount("Card", "MXN", 0)
			item := a.weeklyItem(account, nil)
			tx := a.createTransaction(map[string]any{"type": "expense", "account_id": account.ID, "amount": 5, "occurred_on": daysFromToday(-9)})
			a.do(http.MethodPut, "/api/v1/transactions/"+tx.ID.String()+"/recurring", map[string]any{"recurring_id": item.ID}).expect(http.StatusOK)
			return func() {
				a.do(http.MethodDelete, "/api/v1/transactions/"+tx.ID.String()+"/recurring", nil).expect(http.StatusOK)
			}, []string{finance.ActionTransactionUpdated}
		}},
		{"accept-recurring-suggestion", func(a *testAPI) (func(), []string) {
			account := a.createAccount("Card", "MXN", 0)
			a.seedMonthly(account, "Netflix", nil, 18900, 18900, 18900)
			key := a.suggestions()[0].Key
			return post(a, "/api/v1/recurring/suggestions/accept", map[string]any{"key": key}, http.StatusCreated),
				[]string{finance.ActionRecurringCreated, finance.ActionTransactionUpdated, finance.ActionTransactionUpdated, finance.ActionTransactionUpdated}
		}},
		{"dismiss-recurring-suggestion", func(a *testAPI) (func(), []string) {
			account := a.createAccount("Card", "MXN", 0)
			a.seedMonthly(account, "Netflix", nil, 18900, 18900, 18900)
			key := a.suggestions()[0].Key
			return post(a, "/api/v1/recurring/suggestions/dismiss", map[string]any{"key": key}, http.StatusNoContent),
				[]string{finance.ActionSuggestionDismissed}
		}},
		{"export-data", func(a *testAPI) (func(), []string) {
			return func() { a.do(http.MethodGet, "/api/v1/export?format=csv", nil).expect(http.StatusOK) },
				[]string{finance.ActionExportRequested}
		}},
	}
}

func TestEveryFinanceMutationIsCovered(t *testing.T) {
	t.Parallel()
	raw, err := OpenAPI(Deps{
		Logger:     slog.New(slog.DiscardHandler),
		Auth:       &auth.Service{},
		Finance:    &finance.Service{},
		Workspaces: &workspace.Service{},
		OAuth:      &oauth.Server{},
	})
	if err != nil {
		t.Fatal(err)
	}
	var spec struct {
		Paths map[string]map[string]struct {
			OperationID string   `json:"operationId"`
			Tags        []string `json:"tags"`
		} `json:"paths"`
	}
	if err := json.Unmarshal(raw, &spec); err != nil {
		t.Fatal(err)
	}
	covered := map[string]bool{}
	for _, m := range financeMutations() {
		covered[m.operation] = true
	}
	found := map[string]bool{}
	for path, methods := range spec.Paths {
		for method, op := range methods {
			isFinance := slices.ContainsFunc(op.Tags, func(tag string) bool { return slices.Contains(financeTags, tag) })
			if (isFinance && method != "get") || slices.Contains(tracedReads, op.OperationID) {
				found[op.OperationID] = true
				if !covered[op.OperationID] {
					t.Errorf("%s %s (%s) changes finance data but has no entry in financeMutations: record it in the activity log and add it", strings.ToUpper(method), path, op.OperationID)
				}
			}
		}
	}
	for op := range covered {
		if !found[op] {
			t.Errorf("financeMutations lists %q, which is not a finance mutation of the API", op)
		}
	}
}

func TestFinanceMutationsRecordActivity(t *testing.T) {
	t.Parallel()
	for _, m := range financeMutations() {
		t.Run(m.operation, func(t *testing.T) {
			t.Parallel()
			api := newTestAPI(t)
			do, want := m.prepare(api)
			before := api.activity("?limit=200")
			do()
			after := api.activity("?limit=200")
			added := len(after.Items) - len(before.Items)
			if added != len(want) {
				t.Fatalf("expected %d new entries %v, got %d: %v", len(want), want, added, actions(after))
			}
			got := actions(finance.ActivityPage{Items: after.Items[:added]})
			slices.Sort(got)
			slices.Sort(want)
			if !slices.Equal(got, want) {
				t.Fatalf("expected actions %v, got %v", want, got)
			}
			for _, e := range after.Items[:added] {
				if e.Actor == nil || e.Actor.ID != api.owner.ID || e.Channel != finance.ChannelWeb {
					t.Fatalf("entry should be attributed to the owner on the web: %+v", e)
				}
			}
		})
	}
}
