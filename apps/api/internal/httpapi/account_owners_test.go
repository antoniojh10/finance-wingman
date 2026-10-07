package httpapi

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/google/uuid"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/finance"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/testutil"
)

func (a *testAPI) createOwnedAccount(name, owner string) finance.Account {
	a.t.Helper()
	var account finance.Account
	a.do(http.MethodPost, "/api/v1/accounts", map[string]any{
		"name": name, "type": "checking", "currency": "EUR", "owner": owner, "balance_as_of": "2000-01-01",
	}).expect(http.StatusCreated).decode(&account)
	return account
}

func ownerID(a finance.Account) string {
	if a.Owner == nil {
		return "shared"
	}
	return a.Owner.ID.String()
}

func accountIDs(accounts []finance.Account) map[uuid.UUID]bool {
	ids := map[uuid.UUID]bool{}
	for _, a := range accounts {
		ids[a.ID] = true
	}
	return ids
}

func TestAccountOwner(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	ana := testutil.NewMember(t, api.pool, api.workspace.ID, "ana@example.com", "Ana")

	mine := api.createOwnedAccount("BNP", "")
	if mine.Owner == nil || mine.Owner.ID != api.owner.ID || mine.Owner.Name != "Owner" || mine.Owner.Email != "owner@example.com" {
		t.Fatalf("new accounts should belong to their creator: %+v", mine.Owner)
	}
	if got := ownerID(api.createOwnedAccount("Joint", "shared")); got != "shared" {
		t.Fatalf("expected a shared account, owner %s", got)
	}
	anas := api.createOwnedAccount("bnp", ana.String())
	if ownerID(anas) != ana.String() {
		t.Fatalf("expected Ana's account, owner %s", ownerID(anas))
	}

	tests := []struct {
		name   string
		body   map[string]any
		status int
	}{
		{"same name for the same owner", map[string]any{"name": "BNP", "owner": "me"}, http.StatusConflict},
		{"same name for shared accounts", map[string]any{"name": "joint", "owner": "shared"}, http.StatusConflict},
		{"owner outside the workspace", map[string]any{"name": "X", "owner": uuid.NewString()}, http.StatusUnprocessableEntity},
		{"malformed owner", map[string]any{"name": "X", "owner": "Ana"}, http.StatusUnprocessableEntity},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.body["type"], tc.body["currency"] = "checking", "EUR"
			body := api.do(http.MethodPost, "/api/v1/accounts", tc.body).expectError(tc.status)
			if tc.status == http.StatusUnprocessableEntity && len(body.Errors) == 0 {
				t.Fatalf("expected field errors: %+v", body)
			}
		})
	}
}

func TestUpdateAccountOwner(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	ana := testutil.NewMember(t, api.pool, api.workspace.ID, "ana@example.com", "Ana")
	account := api.createOwnedAccount("Savings", "")
	path := "/api/v1/accounts/" + account.ID.String()

	var updated finance.Account
	api.do(http.MethodPatch, path, map[string]any{"owner": ana.String()}).expect(http.StatusOK).decode(&updated)
	if ownerID(updated) != ana.String() {
		t.Fatalf("expected Ana as owner, got %s", ownerID(updated))
	}
	// Other fields leave the owner alone.
	api.do(http.MethodPatch, path, map[string]any{"name": "Ana's savings"}).expect(http.StatusOK).decode(&updated)
	if ownerID(updated) != ana.String() {
		t.Fatalf("renaming changed the owner to %s", ownerID(updated))
	}
	var shared finance.Account
	api.do(http.MethodPatch, path, map[string]any{"owner": "shared"}).expect(http.StatusOK).decode(&shared)
	if shared.Owner != nil {
		t.Fatalf("expected a shared account, got %+v", shared.Owner)
	}

	api.do(http.MethodPatch, path, map[string]any{"owner": uuid.NewString()}).expectError(http.StatusUnprocessableEntity)
	api.createOwnedAccount("Ana's savings", "me")
	api.do(http.MethodPatch, path, map[string]any{"owner": "me"}).expectError(http.StatusConflict)
}

func TestOwnerFilters(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	ana := testutil.NewMember(t, api.pool, api.workspace.ID, "ana@example.com", "Ana")
	mine := api.createOwnedAccount("Mine", "me")
	anas := api.createOwnedAccount("Ana's", ana.String())
	joint := api.createOwnedAccount("Joint", "shared")
	food := api.createCategory("Food", "expense")

	expense := func(account finance.Account, amount int64) finance.Transaction {
		return api.createTransaction(map[string]any{
			"type": "expense", "account_id": account.ID, "amount": amount, "category_id": food.ID, "occurred_on": "2026-10-01",
		})
	}
	myExpense := expense(mine, 100)
	anasExpense := expense(anas, 200)
	jointExpense := expense(joint, 400)
	transfer := api.createTransaction(map[string]any{
		"type": "transfer", "account_id": anas.ID, "destination_account_id": joint.ID, "amount": 50, "occurred_on": "2026-10-02",
	})
	if transfer.AccountOwner == nil || transfer.AccountOwner.ID != ana || transfer.DestinationOwner != nil {
		t.Fatalf("transactions should carry their accounts' owners: %+v %+v", transfer.AccountOwner, transfer.DestinationOwner)
	}

	t.Run("accounts", func(t *testing.T) {
		for owner, want := range map[string][]finance.Account{
			"":            {mine, anas, joint},
			"me":          {mine},
			"shared":      {joint},
			ana.String():  {anas},
			"Me":          {mine},
			"  shared   ": {joint},
		} {
			var body listBody[finance.Account]
			api.do(http.MethodGet, "/api/v1/accounts?owner="+url.QueryEscape(owner), nil).expect(http.StatusOK).decode(&body)
			got := accountIDs(body.Items)
			if len(got) != len(want) {
				t.Fatalf("owner %q: got %+v", owner, body.Items)
			}
			for _, a := range want {
				if !got[a.ID] {
					t.Fatalf("owner %q: missing account %s", owner, a.Name)
				}
			}
		}
	})

	t.Run("transactions", func(t *testing.T) {
		for owner, want := range map[string][]finance.Transaction{
			"me":         {myExpense},
			"shared":     {jointExpense, transfer},
			ana.String(): {anasExpense, transfer},
		} {
			var page finance.TransactionPage
			api.do(http.MethodGet, "/api/v1/transactions?owner="+owner, nil).expect(http.StatusOK).decode(&page)
			ids := map[uuid.UUID]bool{}
			for _, tx := range page.Items {
				ids[tx.ID] = true
			}
			if len(ids) != len(want) {
				t.Fatalf("owner %q: got %d transactions, want %d", owner, len(ids), len(want))
			}
			for _, tx := range want {
				if !ids[tx.ID] {
					t.Fatalf("owner %q: missing transaction %s", owner, tx.ID)
				}
			}
		}
	})

	t.Run("summary", func(t *testing.T) {
		for owner, want := range map[string]struct{ expense, balance int64 }{
			"":           {700, -700},
			"me":         {100, -100},
			"shared":     {400, -400 + 50},
			ana.String(): {200, -200 - 50},
		} {
			var summary finance.Summary
			api.do(http.MethodGet, "/api/v1/summary?from=2026-10-01&to=2026-10-31&owner="+owner, nil).expect(http.StatusOK).decode(&summary)
			if len(summary.Currencies) != 1 || summary.Currencies[0].Expense != want.expense || summary.Currencies[0].Balance != want.balance {
				t.Fatalf("owner %q: got %+v, want %+v", owner, summary.Currencies, want)
			}
		}
	})

	for _, path := range []string{"/api/v1/accounts", "/api/v1/transactions", "/api/v1/summary"} {
		t.Run("rejects a malformed owner on "+path, func(t *testing.T) {
			api.do(http.MethodGet, path+"?owner=nobody", nil).expectError(http.StatusUnprocessableEntity)
		})
	}
}

// TestRemovedMemberAccountsBecomeShared checks that a member's accounts
// stay in the workspace as shared ones when they leave, renamed when a
// shared account already uses the name.
func TestRemovedMemberAccountsBecomeShared(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	luis := testutil.NewMember(t, api.pool, api.workspace.ID, "luis@example.com", "Luis")
	api.createOwnedAccount("BNP", "shared")
	clashing := api.createOwnedAccount("bnp", luis.String())
	other := api.createOwnedAccount("Wallet", luis.String())

	api.do(http.MethodDelete, "/api/v1/workspaces/"+api.workspace.ID.String()+"/members/"+luis.String(), nil).
		expect(http.StatusNoContent)

	for id, name := range map[string]string{clashing.ID.String(): "bnp (Luis)", other.ID.String(): "Wallet"} {
		account := api.getAccount(id)
		if account.Owner != nil || account.Name != name {
			t.Fatalf("expected shared account %q, got %q owned by %+v", name, account.Name, account.Owner)
		}
	}
}
