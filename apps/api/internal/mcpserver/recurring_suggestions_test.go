package mcpserver

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/finance"
)

// seedMonthly records one expense every 30 days ending 5 days ago.
func (h *harness) seedMonthly(account finance.Account, description string, category *finance.Category, amounts ...int64) []finance.Transaction {
	h.t.Helper()
	var txs []finance.Transaction
	for i, amount := range amounts {
		in := finance.TransactionInput{
			Type: "expense", AccountID: account.ID, Amount: amount, Description: description,
			OccurredOn: h.svc.Today().AddDate(0, 0, -5-30*(len(amounts)-1-i)).Format(time.DateOnly),
		}
		if category != nil {
			in.CategoryID = &category.ID
		}
		tx, err := h.svc.CreateTransaction(context.Background(), in)
		if err != nil {
			h.t.Fatal(err)
		}
		txs = append(txs, tx)
	}
	return txs
}

func (h *harness) suggestions() []suggestionOut {
	h.t.Helper()
	var out listSuggestionsOut
	h.mustCall("list_recurring_suggestions", nil, &out)
	return out.Suggestions
}

func TestListRecurringSuggestions(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	card := h.account("Card", "MXN")
	cat := h.category("Entertainment", "expense")

	text := h.mustCall("list_recurring_suggestions", nil, nil)
	if !strings.Contains(text, "No recurring patterns") {
		t.Fatalf("unexpected empty text: %s", text)
	}

	h.seedMonthly(card, "Spotify", &cat, 11900, 11900, 11900)
	var out listSuggestionsOut
	text = h.mustCall("list_recurring_suggestions", nil, &out)
	if len(out.Suggestions) != 1 {
		t.Fatalf("expected one suggestion: %+v", out)
	}
	s := out.Suggestions[0]
	if s.Name != "Spotify" || s.Type != "expense" || s.Amount != "119.00" || s.Currency != "MXN" || s.Account != "Card" ||
		s.Category != "Entertainment" || s.Frequency != "monthly" || s.Transactions != 3 || s.Key == "" || s.NextDue == "" {
		t.Fatalf("unexpected suggestion: %+v", s)
	}
	if !strings.Contains(text, "Spotify") || !strings.Contains(text, s.Key) || !strings.Contains(text, "Ask the user") {
		t.Fatalf("unexpected text: %s", text)
	}
}

func TestAcceptRecurringSuggestion(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	card := h.account("Card", "MXN")
	ent := h.category("Entertainment", "expense")
	h.category("Streaming", "expense")
	h.seedMonthly(card, "Spotify", &ent, 11900, 11900, 11900)
	key := h.suggestions()[0].Key

	var out acceptSuggestionOut
	text := h.mustCall("accept_recurring_suggestion", map[string]any{
		"key": key, "name": "Spotify Premium", "amount": 129.5, "category": "streaming",
	}, &out)
	r := out.Recurring
	if r.Name != "Spotify Premium" || r.Amount != "129.50" || r.Category != "Streaming" || r.Frequency != "month" || r.Type != "expense" || out.LinkedTransactions != 3 {
		t.Fatalf("unexpected output: %+v", out)
	}
	if !strings.Contains(text, "Spotify Premium") || !strings.Contains(text, "linked 3") {
		t.Fatalf("unexpected text: %s", text)
	}
	if got := h.suggestions(); len(got) != 0 {
		t.Fatalf("accepted suggestion should disappear: %+v", got)
	}
	var tx transactionsOut
	h.mustCall("list_transactions", nil, &tx)
	for _, x := range tx.Transactions {
		if x.RecurringName != "Spotify Premium" {
			t.Fatalf("transactions should be linked: %+v", x)
		}
	}
}

func TestAcceptRecurringSuggestionDefaults(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	card := h.account("Card", "MXN")
	ent := h.category("Entertainment", "expense")
	h.seedMonthly(card, "Gym", &ent, 50000, 50000, 50000)
	var out acceptSuggestionOut
	h.mustCall("accept_recurring_suggestion", map[string]any{"key": h.suggestions()[0].Key}, &out)
	if out.Recurring.Name != "Gym" || out.Recurring.Amount != "500.00" || out.Recurring.Category != "Entertainment" {
		t.Fatalf("defaults: %+v", out)
	}
}

func TestAcceptRecurringSuggestionErrors(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	card := h.account("Card", "MXN")
	h.category("Entertainment", "expense")
	h.category("Salary", "income")
	h.seedMonthly(card, "Spotify", nil, 11900, 11900, 11900)
	key := h.suggestions()[0].Key

	h.mustFail("accept_recurring_suggestion", map[string]any{"key": "nope"}, "list_recurring_suggestions")
	h.mustFail("accept_recurring_suggestion", map[string]any{"key": key, "category": "Nothing"}, "Entertainment")
	h.mustFail("accept_recurring_suggestion", map[string]any{"key": key, "category": "Salary"}, "Entertainment")
	h.mustFail("accept_recurring_suggestion", map[string]any{"key": key, "amount": -1}, "amount")
	if len(h.suggestions()) != 1 {
		t.Fatal("failed calls must not accept the suggestion")
	}

	// Name clash with an existing recurring item.
	h.mustCall("create_recurring", map[string]any{"name": "Spotify", "type": "expense", "amount": 99, "frequency": "month"}, nil)
	h.mustFail("accept_recurring_suggestion", map[string]any{"key": key}, "already exists")
	h.mustCall("accept_recurring_suggestion", map[string]any{"key": key, "name": "Spotify family"}, nil)
}

func TestDismissRecurringSuggestion(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	card := h.account("Card", "MXN")
	h.seedMonthly(card, "Spotify", nil, 11900, 11900, 11900)
	key := h.suggestions()[0].Key

	var out dismissSuggestionOut
	h.mustCall("dismiss_recurring_suggestion", map[string]any{"key": key}, &out)
	if out.Dismissed != key || len(h.suggestions()) != 0 {
		t.Fatalf("suggestion should be gone: %+v", out)
	}
	h.mustCall("dismiss_recurring_suggestion", map[string]any{"key": key}, nil) // idempotent
	h.mustFail("dismiss_recurring_suggestion", map[string]any{"key": "!!"}, "listed suggestion")
}

func TestRecurringMatchHint(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.account("Card", "MXN")
	var item recurringOut
	h.mustCall("create_recurring", map[string]any{"name": "Netflix", "type": "expense", "amount": 189, "frequency": "month", "account": "Card"}, &item)

	var out transactionOut
	text := h.mustCall("add_expense", map[string]any{"amount": 189, "account": "Card", "description": "Netflix.com"}, &out)
	m := out.RecurringMatch
	if m == nil || m.Recurring != "Netflix" || m.RecurringID != item.ID || m.Period == "" || m.EstimatedAmount != "189.00" {
		t.Fatalf("expected a match: %+v", out)
	}
	if out.RecurringID != "" {
		t.Fatalf("must not auto-link: %+v", out)
	}
	if !strings.Contains(text, "NOT linked") || !strings.Contains(text, "link_transaction_to_recurring") {
		t.Fatalf("unexpected text: %s", text)
	}

	// No match: other description, other type.
	out = transactionOut{}
	text = h.mustCall("add_expense", map[string]any{"amount": 50, "account": "Card", "description": "Tacos"}, &out)
	if out.RecurringMatch != nil || strings.Contains(text, "recurring") {
		t.Fatalf("unexpected match: %s %+v", text, out)
	}
	out = transactionOut{}
	h.mustCall("add_income", map[string]any{"amount": 189, "account": "Card", "description": "Netflix refund"}, &out)
	if out.RecurringMatch != nil {
		t.Fatalf("income must not match an expense item: %+v", out)
	}
}

func TestLinkTransactionToRecurring(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.account("Card", "MXN")
	h.account("Other", "MXN")
	h.mustCall("create_recurring", map[string]any{"name": "Netflix", "type": "expense", "amount": 189, "frequency": "month", "account": "Card"}, nil)
	var tx transactionOut
	h.mustCall("add_expense", map[string]any{"amount": 189, "account": "Card", "description": "Netflix"}, &tx)

	var out linkRecurringOut
	text := h.mustCall("link_transaction_to_recurring", map[string]any{"transaction": tx.ID, "recurring": "netflix"}, &out)
	if out.Transaction.RecurringName != "Netflix" || out.Transaction.RecurringDueOn == "" || !strings.Contains(text, "Linked") {
		t.Fatalf("link: %s %+v", text, out)
	}
	period := out.Transaction.RecurringDueOn

	out = linkRecurringOut{}
	text = h.mustCall("link_transaction_to_recurring", map[string]any{"transaction": tx.ID, "unlink": true}, &out)
	if out.Transaction.RecurringID != "" || !strings.Contains(text, "no longer linked") {
		t.Fatalf("unlink: %s %+v", text, out)
	}
	out = linkRecurringOut{}
	h.mustCall("link_transaction_to_recurring", map[string]any{"transaction": tx.ID, "recurring": "Netflix", "period": period}, &out)
	if out.Transaction.RecurringDueOn != period {
		t.Fatalf("period: %+v", out)
	}

	h.mustFail("link_transaction_to_recurring", map[string]any{"transaction": "abc", "recurring": "Netflix"}, "ID of a transaction")
	h.mustFail("link_transaction_to_recurring", map[string]any{"transaction": tx.ID}, "unlink=true")
	h.mustFail("link_transaction_to_recurring", map[string]any{"transaction": tx.ID, "recurring": "Spotify"}, "Netflix")
	h.mustFail("link_transaction_to_recurring", map[string]any{"transaction": tx.ID, "recurring": "Netflix", "period": "2020-01-02"}, "period")
	h.mustFail("link_transaction_to_recurring", map[string]any{"transaction": tx.ID, "unlink": true, "recurring": "Netflix"}, "unlink=true")
	h.mustFail("link_transaction_to_recurring", map[string]any{"transaction": "00000000-0000-0000-0000-000000000000", "recurring": "Netflix"}, "not found")

	var other transactionOut
	h.mustCall("add_expense", map[string]any{"amount": 10, "account": "Other", "description": "Netflix"}, &other)
	h.mustFail("link_transaction_to_recurring", map[string]any{"transaction": other.ID, "recurring": "Netflix"}, "account")
}
