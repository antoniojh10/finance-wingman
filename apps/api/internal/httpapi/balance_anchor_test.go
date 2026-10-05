package httpapi

import (
	"net/http"
	"testing"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/finance"
)

func (a *testAPI) createAnchoredAccount(name, currency string, initialBalance int64, asOf string) finance.Account {
	a.t.Helper()
	body := map[string]any{"name": name, "type": "checking", "currency": currency, "initial_balance": initialBalance}
	if asOf != "" {
		body["balance_as_of"] = asOf
	}
	var account finance.Account
	a.do(http.MethodPost, "/api/v1/accounts", body).expect(http.StatusCreated).decode(&account)
	return account
}

func TestBalanceAnchorDefaultsToToday(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	account := api.createAnchoredAccount("Main", "EUR", 50000, "")
	if account.BalanceAsOf != daysFromToday(0) {
		t.Fatalf("expected default anchor today, got %s", account.BalanceAsOf)
	}
}

func TestBackdatedTransactionDoesNotChangeBalance(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	// Today the account holds 500; an income of 200 from a month ago is already part of it.
	account := api.createAnchoredAccount("Main", "EUR", 50000, "")
	api.createTransaction(map[string]any{"type": "income", "account_id": account.ID, "amount": 20000, "occurred_on": daysFromToday(-30)})
	if got := api.getAccount(account.ID.String()).Balance; got != 50000 {
		t.Fatalf("backdated income changed the balance: %d", got)
	}
}

func TestSameDayTransactionIsExcludedAndLaterOnesCount(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	account := api.createAnchoredAccount("Main", "EUR", 50000, "2026-06-10")
	id := account.ID.String()

	api.createTransaction(map[string]any{"type": "expense", "account_id": account.ID, "amount": 1000, "occurred_on": "2026-06-10"})
	if got := api.getAccount(id).Balance; got != 50000 {
		t.Fatalf("same-day expense must be excluded, balance = %d", got)
	}
	api.createTransaction(map[string]any{"type": "expense", "account_id": account.ID, "amount": 700, "occurred_on": "2026-06-11"})
	api.createTransaction(map[string]any{"type": "income", "account_id": account.ID, "amount": 200, "occurred_on": "2026-06-12"})
	if got := api.getAccount(id).Balance; got != 50000-700+200 {
		t.Fatalf("balance = %d", got)
	}

	var list struct{ Items []finance.Account }
	api.do(http.MethodGet, "/api/v1/accounts", nil).expect(http.StatusOK).decode(&list)
	if list.Items[0].Balance != 50000-700+200 {
		t.Fatalf("list balance = %d", list.Items[0].Balance)
	}
}

func TestTransferUsesEachAccountsAnchor(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	source := api.createAnchoredAccount("Source", "MXN", 100000, "2026-06-01")
	dest := api.createAnchoredAccount("Dest", "MXN", 0, "2026-06-20")

	// Between both anchors: counts for the source only.
	api.createTransaction(map[string]any{"type": "transfer", "account_id": source.ID, "amount": 30000, "destination_account_id": dest.ID, "occurred_on": "2026-06-10"})
	if got := api.getAccount(source.ID.String()).Balance; got != 70000 {
		t.Fatalf("source balance = %d", got)
	}
	if got := api.getAccount(dest.ID.String()).Balance; got != 0 {
		t.Fatalf("dest balance = %d", got)
	}
	// Same day as the destination anchor: excluded there, counted at the source.
	api.createTransaction(map[string]any{"type": "transfer", "account_id": source.ID, "amount": 5000, "destination_account_id": dest.ID, "occurred_on": "2026-06-20"})
	// After both anchors: counts on both sides.
	api.createTransaction(map[string]any{"type": "transfer", "account_id": source.ID, "amount": 1000, "destination_account_id": dest.ID, "occurred_on": "2026-06-21"})
	if got := api.getAccount(source.ID.String()).Balance; got != 100000-30000-5000-1000 {
		t.Fatalf("source balance = %d", got)
	}
	if got := api.getAccount(dest.ID.String()).Balance; got != 1000 {
		t.Fatalf("dest balance = %d", got)
	}
}

func TestSummaryBalanceUsesAnchorButPeriodTotalsKeepBackdated(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	account := api.createAnchoredAccount("Main", "EUR", 50000, "2026-09-15")
	api.createTransaction(map[string]any{"type": "income", "account_id": account.ID, "amount": 20000, "occurred_on": "2026-09-01"})
	api.createTransaction(map[string]any{"type": "expense", "account_id": account.ID, "amount": 3000, "occurred_on": "2026-09-20"})

	var summary finance.Summary
	api.do(http.MethodGet, "/api/v1/summary?from=2026-09-01&to=2026-09-30", nil).expect(http.StatusOK).decode(&summary)
	eur := summary.Currencies[0]
	if eur.Balance != 50000-3000 {
		t.Fatalf("summary balance = %d", eur.Balance)
	}
	if eur.Income != 20000 || eur.Expense != 3000 {
		t.Fatalf("period totals must keep backdated transactions: %+v", eur)
	}
}

func TestUpdateAccountAnchor(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	account := api.createAnchoredAccount("Main", "EUR", 50000, "2026-06-10")
	api.createTransaction(map[string]any{"type": "income", "account_id": account.ID, "amount": 200, "occurred_on": "2026-06-15"})
	path := "/api/v1/accounts/" + account.ID.String()

	// Editing the balance alone keeps the anchor.
	var updated finance.Account
	api.do(http.MethodPatch, path, map[string]any{"initial_balance": 60000}).expect(http.StatusOK).decode(&updated)
	if updated.BalanceAsOf != "2026-06-10" || updated.Balance != 60200 {
		t.Fatalf("anchor should be kept: %+v", updated)
	}

	// Moving the anchor past the transaction excludes it.
	api.do(http.MethodPatch, path, map[string]any{"balance_as_of": "2026-06-15"}).expect(http.StatusOK).decode(&updated)
	if updated.BalanceAsOf != "2026-06-15" || updated.Balance != 60000 || updated.InitialBalance != 60000 {
		t.Fatalf("unexpected after moving anchor: %+v", updated)
	}

	api.do(http.MethodPatch, path, map[string]any{"balance_as_of": "15/06/2026"}).expect(http.StatusUnprocessableEntity)
	api.do(http.MethodPost, "/api/v1/accounts", map[string]any{"name": "X", "type": "cash", "currency": "EUR", "balance_as_of": "nope"}).
		expect(http.StatusUnprocessableEntity)
}
