package mcpserver

import (
	"strings"
	"testing"
	"time"
)

func TestMCPBackdatedTransactionKeepsBalance(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	today := time.Now().UTC().Format(time.DateOnly)

	var created accountOut
	h.mustCall("create_account", map[string]any{"name": "Main", "type": "checking", "currency": "EUR", "initial_balance": 500}, &created)
	if created.BalanceAsOf != today {
		t.Fatalf("expected default anchor %s, got %s", today, created.BalanceAsOf)
	}

	month := time.Now().UTC().AddDate(0, 0, -30).Format(time.DateOnly)
	h.mustCall("add_income", map[string]any{"amount": 200, "account": "Main", "date": month}, nil)

	var list accountsOut
	h.mustCall("list_accounts", nil, &list)
	if list.Accounts[0].Balance != "500.00" {
		t.Fatalf("backdated income must not change the balance: %+v", list.Accounts[0])
	}
	var summary summaryOut
	h.mustCall("get_summary", map[string]any{"from": month, "to": month}, &summary)
	if summary.Currencies[0].Income != "200.00" || summary.Currencies[0].Balance != "500.00" {
		t.Fatalf("summary: %+v", summary.Currencies[0])
	}
}

func TestMCPEditableAnchor(t *testing.T) {
	t.Parallel()
	h := newHarness(t)

	var out accountOut
	h.mustCall("create_account", map[string]any{"name": "Main", "type": "checking", "currency": "EUR", "initial_balance": 500, "balance_as_of": "2026-06-10"}, &out)
	if out.BalanceAsOf != "2026-06-10" {
		t.Fatalf("anchor not applied: %+v", out)
	}
	h.mustCall("add_income", map[string]any{"amount": 200, "account": "Main", "date": "2026-06-10"}, nil)
	h.mustCall("add_expense", map[string]any{"amount": 50, "account": "Main", "date": "2026-06-11"}, nil)

	h.mustCall("update_account", map[string]any{"account": "Main", "initial_balance": 600}, &out)
	if out.Balance != "550.00" || out.BalanceAsOf != "2026-06-10" {
		t.Fatalf("editing the balance must keep the anchor: %+v", out)
	}
	h.mustCall("update_account", map[string]any{"account": "Main", "balance_as_of": "2026-06-11"}, &out)
	if out.Balance != "600.00" || out.BalanceAsOf != "2026-06-11" {
		t.Fatalf("moving the anchor: %+v", out)
	}
	h.mustFail("update_account", map[string]any{"account": "Main", "balance_as_of": "yesterday"}, "balance_as_of")
	h.mustFail("create_account", map[string]any{"name": "Other", "type": "cash", "currency": "EUR", "balance_as_of": "06/10/2026"}, "balance_as_of")

	var batch accountsOut
	h.mustCall("create_accounts", map[string]any{"items": []map[string]any{
		{"name": "A", "type": "cash", "currency": "EUR", "initial_balance": 10, "balance_as_of": "2026-01-01"},
	}}, &batch)
	if batch.Accounts[0].BalanceAsOf != "2026-01-01" {
		t.Fatalf("batch anchor: %+v", batch.Accounts[0])
	}
}

func TestMCPInstructionsPointToWebExport(t *testing.T) {
	t.Parallel()
	if !strings.Contains(instructions, "Export your data") {
		t.Fatal("server instructions must say that exports live in the web app")
	}
}

func TestMCPInstructionsExplainAnchor(t *testing.T) {
	t.Parallel()
	if !strings.Contains(instructions, "balance_as_of") {
		t.Fatal("server instructions must explain balance_as_of")
	}
}
