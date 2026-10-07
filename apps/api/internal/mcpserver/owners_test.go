package mcpserver

import (
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/finance"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/testutil"
)

// ownersHarness acts as Luis in a workspace he shares with Ana.
func ownersHarness(t *testing.T) (h *harness, luis, ana uuid.UUID) {
	t.Helper()
	h = newHarness(t)
	luis = testutil.NewMember(t, h.pool, h.workspaceID, "luis@example.com", "Luis")
	ana = testutil.NewMember(t, h.pool, h.workspaceID, "ana@example.com", "Ana")
	h.actor = luis
	return h, luis, ana
}

func (h *harness) ownedAccount(name string, owner uuid.UUID) finance.Account {
	h.t.Helper()
	in := finance.CreateAccountInput{Name: name, Type: "checking", Currency: "EUR", BalanceAsOf: "2000-01-01", Owner: finance.OwnerShared}
	if owner != uuid.Nil {
		in.Owner = owner.String()
	}
	a, err := h.svc.CreateAccount(h.ctx, in)
	if err != nil {
		h.t.Fatal(err)
	}
	return a
}

func TestCreateAccountOwner(t *testing.T) {
	t.Parallel()
	h, _, ana := ownersHarness(t)

	var out accountOut
	text := h.mustCall("create_account", map[string]any{"name": "BNP", "type": "checking", "currency": "EUR"}, &out)
	if out.Owner != "Luis" || !strings.Contains(text, "(owner: Luis)") {
		t.Fatalf("new accounts should belong to the caller: %+v %s", out, text)
	}
	h.mustCall("create_account", map[string]any{"name": "BNP", "type": "checking", "currency": "EUR", "owner": "ana"}, &out)
	if out.Owner != "Ana" {
		t.Fatalf("expected Ana's account: %+v", out)
	}
	h.mustCall("create_account", map[string]any{"name": "BNP", "type": "checking", "currency": "EUR", "owner": "Shared"}, &out)
	if out.Owner != "shared" {
		t.Fatalf("expected a shared account: %+v", out)
	}
	h.mustCall("create_account", map[string]any{"name": "Cash", "type": "cash", "currency": "EUR", "owner": "ana@example.com"}, &out)
	if out.Owner != "Ana" {
		t.Fatalf("owner by email: %+v", out)
	}

	h.mustFail("create_account", map[string]any{"name": "BNP", "type": "checking", "currency": "EUR", "owner": "me"}, "already exists for this owner")
	h.mustFail("create_account", map[string]any{"name": "X", "type": "cash", "currency": "EUR", "owner": "Eve"},
		`no workspace member matches owner "Eve"; use "me", "shared" or one of: Ana <ana@example.com>, Luis <luis@example.com>`)

	var batch accountsOut
	h.mustCall("create_accounts", map[string]any{"items": []map[string]any{
		{"name": "Savings", "type": "savings", "currency": "EUR"},
		{"name": "Savings", "type": "savings", "currency": "EUR", "owner": ana.String()},
	}}, &batch)
	if batch.Accounts[0].Owner != "Luis" || batch.Accounts[1].Owner != "Ana" {
		t.Fatalf("unexpected batch owners: %+v", batch.Accounts)
	}
	h.mustFail("create_accounts", map[string]any{"items": []map[string]any{
		{"name": "Card", "type": "credit_card", "currency": "EUR", "owner": "Nobody"},
	}}, "items[0]")
}

func TestAccountsWithTheSameName(t *testing.T) {
	t.Parallel()
	h, luis, ana := ownersHarness(t)
	mine := h.ownedAccount("BNP", luis)
	anas := h.ownedAccount("BNP", ana)

	var tx transactionOut
	h.mustCall("add_expense", map[string]any{"amount": 10, "account": "bnp"}, &tx)
	if tx.Account != "BNP (Luis)" {
		t.Fatalf("a bare name should pick the caller's account, got %q", tx.Account)
	}
	h.mustCall("add_expense", map[string]any{"amount": 10, "account": "BNP (Ana)"}, &tx)
	if tx.Account != "BNP (Ana)" {
		t.Fatalf("name with owner should pick Ana's account, got %q", tx.Account)
	}
	h.mustCall("add_transfer", map[string]any{"amount": 5, "from_account": "BNP", "to_account": anas.ID.String()}, &tx)
	if tx.Account != "BNP (Luis)" || tx.ToAccount != "BNP (Ana)" {
		t.Fatalf("unexpected transfer: %+v", tx)
	}

	// Without an account of their own, the caller has to pick one.
	h.actor = uuid.Nil
	h.mustFail("add_expense", map[string]any{"amount": 10, "account": "BNP"}, `several accounts match "BNP": BNP [Ana, id `+anas.ID.String()+`], BNP [Luis, id `+mine.ID.String()+`]`)

	var accounts accountsOut
	text := h.mustCall("list_accounts", nil, &accounts)
	if !strings.Contains(text, "- BNP (Ana, checking, EUR)") || !strings.Contains(text, "- BNP (Luis, checking, EUR)") {
		t.Fatalf("list_accounts should show owners: %s", text)
	}
	h.mustFail("add_expense", map[string]any{"amount": 10, "account": "Nope"}, "available accounts: BNP (EUR, Ana), BNP (EUR, Luis)")
}

func TestUpdateAccountOwnerTool(t *testing.T) {
	t.Parallel()
	h, luis, _ := ownersHarness(t)
	h.ownedAccount("Savings", luis)

	var out accountOut
	h.mustCall("update_account", map[string]any{"account": "Savings", "owner": "Ana"}, &out)
	if out.Owner != "Ana" {
		t.Fatalf("expected Ana as owner: %+v", out)
	}
	text := h.mustCall("update_account", map[string]any{"account": "Savings (Ana)", "owner": "shared"}, &out)
	if out.Owner != "shared" || !strings.Contains(text, "(owner: shared)") {
		t.Fatalf("expected a shared account: %+v %s", out, text)
	}
	h.mustFail("update_account", map[string]any{"account": "Savings", "owner": ""}, "owner must not be empty")
	h.mustFail("update_account", map[string]any{"account": "Savings", "owner": "Eve"}, "no workspace member matches")
}

func TestOwnerFilterTools(t *testing.T) {
	t.Parallel()
	h, luis, ana := ownersHarness(t)
	h.ownedAccount("BNP", luis)
	h.ownedAccount("BNP", ana)
	h.ownedAccount("Joint", uuid.Nil)
	h.mustCall("add_expense", map[string]any{"amount": 10, "account": "BNP"}, nil)
	h.mustCall("add_expense", map[string]any{"amount": 20, "account": "BNP (Ana)"}, nil)
	h.mustCall("add_expense", map[string]any{"amount": 40, "account": "Joint"}, nil)

	for owner, want := range map[string]struct {
		accounts int
		expense  string
	}{
		"":       {3, "70.00"},
		"me":     {1, "10.00"},
		"Ana":    {1, "20.00"},
		"shared": {1, "40.00"},
	} {
		var accounts accountsOut
		h.mustCall("list_accounts", map[string]any{"owner": owner}, &accounts)
		if len(accounts.Accounts) != want.accounts {
			t.Fatalf("list_accounts owner %q: %+v", owner, accounts.Accounts)
		}
		var txs transactionsOut
		h.mustCall("list_transactions", map[string]any{"owner": owner}, &txs)
		if len(txs.Transactions) != want.accounts {
			t.Fatalf("list_transactions owner %q: %+v", owner, txs.Transactions)
		}
		var sum summaryOut
		h.mustCall("get_summary", map[string]any{"owner": owner}, &sum)
		if len(sum.Currencies) != 1 || sum.Currencies[0].Expense != want.expense {
			t.Fatalf("get_summary owner %q: %+v", owner, sum.Currencies)
		}
	}

	text := h.mustCall("list_accounts", map[string]any{"owner": "ana"}, nil)
	if !strings.Contains(text, "BNP (Ana, checking, EUR)") {
		t.Fatalf("unexpected list: %s", text)
	}
	for _, tool := range []string{"list_accounts", "list_transactions", "get_summary"} {
		h.mustFail(tool, map[string]any{"owner": "Eve"}, "no workspace member matches owner")
	}
	h.actor = uuid.Nil
	h.mustFail("list_accounts", map[string]any{"owner": "me"}, "needs an authenticated user")
}
