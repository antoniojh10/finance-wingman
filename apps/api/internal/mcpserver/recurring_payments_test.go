package mcpserver

import (
	"strings"
	"testing"
	"time"
)

func (h *harness) weekly(name string, amount float64) recurringOut {
	h.t.Helper()
	var out recurringOut
	h.mustCall("create_recurring", map[string]any{
		"name": name, "type": "expense", "amount": amount, "frequency": "week", "account": "Card",
	}, &out)
	return out
}

func TestMarkRecurringPaid(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.account("Card", "MXN")
	h.category("Entertainment", "expense")
	h.mustCall("create_recurring", map[string]any{
		"name": "Netflix", "type": "expense", "amount": 189, "frequency": "week", "category": "Entertainment",
	}, nil)
	today := h.svc.Today().Format(time.DateOnly)

	var out markRecurringPaidOut
	text := h.mustCall("mark_recurring_paid", map[string]any{"recurring": "netflix"}, &out)
	tx := out.Transaction
	if out.Recurring != "Netflix" || out.Period != today || tx.Type != "expense" || tx.Amount != "189.00" ||
		tx.Account != "Card" || tx.Category != "Entertainment" || tx.Description != "Netflix" || tx.Date != today ||
		tx.RecurringName != "Netflix" || tx.RecurringID == "" {
		t.Fatalf("unexpected output: %+v", out)
	}
	if !strings.Contains(text, "Marked Netflix as paid for "+today) {
		t.Fatalf("unexpected text: %s", text)
	}

	var list listRecurringOut
	h.mustCall("list_recurring", nil, &list)
	it := list.Items[0]
	if it.CurrentPeriod == nil || it.CurrentPeriod.Status != "paid" || it.CurrentPeriod.DueOn != today ||
		it.LastPayment == nil || it.LastPayment.Amount != "189.00" || it.LastPayment.DueOn != today || it.LastPayment.TransactionID != tx.ID {
		t.Fatalf("list_recurring should show the payment: %+v", it)
	}
	if it.Amount != "189.00" {
		t.Fatalf("estimate must not change: %+v", it)
	}
}

func TestMarkRecurringPaidOverrides(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.account("Card", "MXN")
	h.weekly("Gym", 300)
	today := h.svc.Today()
	next := today.AddDate(0, 0, 7).Format(time.DateOnly)
	date := today.AddDate(0, 0, -1).Format(time.DateOnly)

	var out markRecurringPaidOut
	h.mustCall("mark_recurring_paid", map[string]any{"recurring": "Gym", "amount": 275.5, "date": date}, &out)
	if out.Transaction.Amount != "275.50" || out.Transaction.Date != date || out.Period != today.Format(time.DateOnly) {
		t.Fatalf("amount and date overrides: %+v", out)
	}

	// Explicit period pays in advance.
	h.mustCall("mark_recurring_paid", map[string]any{"recurring": "Gym", "period": next}, &out)
	if out.Period != next || out.Transaction.RecurringDueOn != next {
		t.Fatalf("advance payment: %+v", out)
	}
	// Explicit period already paid is honoured: second charge.
	h.mustCall("mark_recurring_paid", map[string]any{"recurring": "Gym", "period": next}, &out)
	if out.Period != next {
		t.Fatalf("second payment: %+v", out)
	}
}

func TestMarkRecurringPaidAlreadyPaid(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.account("Card", "MXN")
	h.weekly("Gym", 300)
	today := h.svc.Today()
	due := today.Format(time.DateOnly)

	h.mustCall("mark_recurring_paid", map[string]any{"recurring": "Gym"}, nil)
	text, isErr := h.call("mark_recurring_paid", map[string]any{"recurring": "Gym"}, nil)
	if !isErr {
		t.Fatalf("second payment of the same period should fail: %s", text)
	}
	next := today.AddDate(0, 0, 7).Format(time.DateOnly)
	for _, want := range []string{"already paid for " + due, "period=" + next + " to pay the next period in advance", "period=" + due, "Ask the user"} {
		if !strings.Contains(text, want) {
			t.Fatalf("error should contain %q: %s", want, text)
		}
	}
	var txs transactionsOut
	h.mustCall("list_transactions", nil, &txs)
	if txs.Total != 1 {
		t.Fatalf("a rejected payment must not create a transaction: %+v", txs)
	}
}

func TestMarkRecurringPaidErrors(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.mustFail("mark_recurring_paid", map[string]any{"recurring": "Gym"}, "no recurring items")

	h.account("Card", "MXN")
	h.weekly("Gym", 300)
	h.weekly("Rent", 1000)
	h.mustFail("mark_recurring_paid", map[string]any{"recurring": "Netflix"}, "available items: Gym (active), Rent (active)")
	h.mustFail("mark_recurring_paid", map[string]any{"recurring": "Gym", "amount": 0}, "amount")
	h.mustFail("mark_recurring_paid", map[string]any{"recurring": "Gym", "date": "yesterday"}, "date")
	h.mustFail("mark_recurring_paid", map[string]any{"recurring": "Gym", "period": "2000-01-02"}, "not a due date")

	h.mustCall("update_recurring", map[string]any{"recurring": "Gym", "status": "paused"}, nil)
	h.mustFail("mark_recurring_paid", map[string]any{"recurring": "Gym"}, "paused")
	h.mustCall("update_recurring", map[string]any{"recurring": "Gym", "status": "cancelled"}, nil)
	h.mustFail("mark_recurring_paid", map[string]any{"recurring": "Gym"}, "cancelled")

	var txs transactionsOut
	h.mustCall("list_transactions", nil, &txs)
	if txs.Total != 0 {
		t.Fatalf("failed payments must not create transactions: %+v", txs)
	}
}

func TestListUpcomingRecurring(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.account("Card", "MXN")
	today := h.svc.Today()
	lateDue := today.AddDate(0, 0, -3).Format(time.DateOnly)
	for _, item := range []map[string]any{
		{"name": "Late", "type": "expense", "amount": 50, "frequency": "week", "start_date": lateDue},
		{"name": "Soon", "type": "expense", "amount": 20, "frequency": "month", "start_date": today.AddDate(0, 0, 5).Format(time.DateOnly)},
		{"name": "Far", "type": "income", "amount": 900, "frequency": "month", "start_date": today.AddDate(0, 0, 20).Format(time.DateOnly)},
	} {
		h.mustCall("create_recurring", item, nil)
	}

	var out upcomingRecurringOut
	text := h.mustCall("list_upcoming_recurring", map[string]any{"days": 7}, &out)
	if len(out.Items) < 2 || out.Items[0].Name != "Late" || out.Items[0].DueOn != lateDue || out.Items[0].Status != "overdue" ||
		out.Items[0].Amount != "50.00" || out.Items[0].Currency != "MXN" {
		t.Fatalf("overdue item first: %+v", out.Items)
	}
	for _, it := range out.Items {
		if it.Name == "Far" {
			t.Fatalf("Far is outside 7 days: %+v", out.Items)
		}
	}
	if !strings.Contains(text, "overdue: Late") || !strings.Contains(text, "pending: Soon") {
		t.Fatalf("unexpected text: %s", text)
	}

	// Paying the overdue period removes it; default window is 30 days.
	h.mustCall("mark_recurring_paid", map[string]any{"recurring": "Late", "period": lateDue}, nil)
	h.mustCall("list_upcoming_recurring", nil, &out)
	foundFar := false
	for _, it := range out.Items {
		if it.Name == "Late" && it.DueOn == lateDue && it.Status != "paid" {
			t.Fatalf("paid period should not stay overdue: %+v", it)
		}
		if it.Name == "Far" {
			foundFar = it.Status == "pending" && it.Type == "income"
		}
	}
	if !foundFar {
		t.Fatalf("default 30 days should include Far: %+v", out.Items)
	}

	h.mustFail("list_upcoming_recurring", map[string]any{"days": 400}, "between 1 and 366")
	h.mustFail("list_upcoming_recurring", map[string]any{"days": -1}, "between 1 and 366")
}

func TestListUpcomingRecurringEmpty(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	var out upcomingRecurringOut
	text := h.mustCall("list_upcoming_recurring", nil, &out)
	if len(out.Items) != 0 || !strings.Contains(text, "Nothing is due") {
		t.Fatalf("unexpected: %s %+v", text, out)
	}
}
