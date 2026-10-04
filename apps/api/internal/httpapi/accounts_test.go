package httpapi

import (
	"net/http"
	"testing"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/finance"
)

func TestListCurrencies(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)

	var body struct{ Items []finance.Currency }
	api.do(http.MethodGet, "/api/v1/currencies", nil).expect(http.StatusOK).decode(&body)

	found := map[string]int{}
	for _, c := range body.Items {
		found[c.Code] = c.MinorUnits
	}
	if found["MXN"] != 2 || found["USD"] != 2 {
		t.Fatalf("expected MXN and USD with 2 minor units, got %v", found)
	}
	if units, ok := found["JPY"]; !ok || units != 0 {
		t.Fatalf("expected JPY with 0 minor units, got %v", found)
	}
}

func TestCreateAccount(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)

	var account finance.Account
	api.do(http.MethodPost, "/api/v1/accounts", map[string]any{
		"name": "  BBVA  ", "type": "credit_card", "currency": "usd", "initial_balance": -5000,
	}).expect(http.StatusCreated).decode(&account)

	if account.Name != "BBVA" || account.Currency != "USD" || account.Type != "credit_card" {
		t.Fatalf("unexpected account: %+v", account)
	}
	if account.InitialBalance != -5000 || account.Balance != -5000 || account.MinorUnits != 2 {
		t.Fatalf("unexpected balances: %+v", account)
	}
}

func TestCreateAccountValidation(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	api.createAccount("Main", "MXN", 0)

	tests := []struct {
		name   string
		body   any
		status int
	}{
		{"missing fields", map[string]any{}, http.StatusUnprocessableEntity},
		{"blank name", map[string]any{"name": "   ", "type": "cash", "currency": "MXN"}, http.StatusUnprocessableEntity},
		{"invalid type", map[string]any{"name": "X", "type": "piggy", "currency": "MXN"}, http.StatusUnprocessableEntity},
		{"unknown currency", map[string]any{"name": "X", "type": "cash", "currency": "XYZ"}, http.StatusUnprocessableEntity},
		{"malformed json", `{"name":`, http.StatusBadRequest},
		{"duplicate name", map[string]any{"name": "main", "type": "cash", "currency": "USD"}, http.StatusConflict},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			api.do(http.MethodPost, "/api/v1/accounts", tc.body).expect(tc.status)
		})
	}
}

func TestGetAccount(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	created := api.createAccount("Wallet", "MXN", 1000)

	got := api.getAccount(created.ID.String())
	if got.ID != created.ID || got.Balance != 1000 {
		t.Fatalf("unexpected account: %+v", got)
	}

	api.do(http.MethodGet, "/api/v1/accounts/"+missingID, nil).expectError(http.StatusNotFound)
	api.do(http.MethodGet, "/api/v1/accounts/not-a-uuid", nil).expect(http.StatusUnprocessableEntity)
}

func TestListAccounts(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	api.createAccount("b account", "MXN", 0)
	archived := api.createAccount("A account", "USD", 0)
	api.do(http.MethodPatch, "/api/v1/accounts/"+archived.ID.String(), map[string]any{"archived": true}).expect(http.StatusOK)

	var active struct{ Items []finance.Account }
	api.do(http.MethodGet, "/api/v1/accounts", nil).expect(http.StatusOK).decode(&active)
	if len(active.Items) != 1 || active.Items[0].Name != "b account" {
		t.Fatalf("expected only the active account, got %+v", active.Items)
	}

	var all struct{ Items []finance.Account }
	api.do(http.MethodGet, "/api/v1/accounts?include_archived=true", nil).expect(http.StatusOK).decode(&all)
	if len(all.Items) != 2 || !all.Items[1].Archived {
		t.Fatalf("expected archived account last, got %+v", all.Items)
	}
}

func TestAccountBalance(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	checking := api.createAccount("Checking", "MXN", 100000)
	savings := api.createAccount("Savings", "MXN", 0)
	usd := api.createAccount("Dollars", "USD", 0)

	api.createTransaction(map[string]any{"type": "income", "account_id": checking.ID, "amount": 50000})
	api.createTransaction(map[string]any{"type": "expense", "account_id": checking.ID, "amount": 12345})
	api.createTransaction(map[string]any{"type": "transfer", "account_id": checking.ID, "amount": 20000, "destination_account_id": savings.ID})
	api.createTransaction(map[string]any{"type": "transfer", "account_id": checking.ID, "amount": 17000, "destination_account_id": usd.ID, "destination_amount": 1000})

	if got := api.getAccount(checking.ID.String()).Balance; got != 100000+50000-12345-20000-17000 {
		t.Fatalf("checking balance = %d", got)
	}
	if got := api.getAccount(savings.ID.String()).Balance; got != 20000 {
		t.Fatalf("savings balance = %d", got)
	}
	if got := api.getAccount(usd.ID.String()).Balance; got != 1000 {
		t.Fatalf("usd balance = %d", got)
	}
}

func TestUpdateAccount(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	account := api.createAccount("Old name", "MXN", 0)
	api.createAccount("Taken", "MXN", 0)
	path := "/api/v1/accounts/" + account.ID.String()

	var updated finance.Account
	api.do(http.MethodPatch, path, map[string]any{"name": "New name", "type": "savings", "initial_balance": 700}).
		expect(http.StatusOK).decode(&updated)
	if updated.Name != "New name" || updated.Type != "savings" || updated.Balance != 700 || updated.Currency != "MXN" {
		t.Fatalf("unexpected update result: %+v", updated)
	}

	api.do(http.MethodPatch, path, map[string]any{"archived": true}).expect(http.StatusOK).decode(&updated)
	if !updated.Archived {
		t.Fatal("expected account to be archived")
	}
	api.do(http.MethodPatch, path, map[string]any{"archived": false}).expect(http.StatusOK).decode(&updated)
	if updated.Archived {
		t.Fatal("expected account to be restored")
	}

	api.do(http.MethodPatch, path, map[string]any{"name": "taken"}).expectError(http.StatusConflict)
	api.do(http.MethodPatch, path, map[string]any{"type": "bogus"}).expect(http.StatusUnprocessableEntity)
	api.do(http.MethodPatch, "/api/v1/accounts/"+missingID, map[string]any{"name": "x"}).expectError(http.StatusNotFound)
}

func TestDeleteAccount(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	empty := api.createAccount("Empty", "MXN", 0)
	used := api.createAccount("Used", "MXN", 0)
	api.createTransaction(map[string]any{"type": "expense", "account_id": used.ID, "amount": 100})

	api.do(http.MethodDelete, "/api/v1/accounts/"+empty.ID.String(), nil).expect(http.StatusNoContent)
	api.do(http.MethodGet, "/api/v1/accounts/"+empty.ID.String(), nil).expect(http.StatusNotFound)

	api.do(http.MethodDelete, "/api/v1/accounts/"+used.ID.String(), nil).expectError(http.StatusConflict)
	api.do(http.MethodDelete, "/api/v1/accounts/"+missingID, nil).expectError(http.StatusNotFound)
}
