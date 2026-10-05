package mcpserver

import (
	"strings"
	"testing"
)

func TestCreateRecurring(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.account("Card", "MXN")
	h.category("Entertainment", "expense")

	var out recurringOut
	text := h.mustCall("create_recurring", map[string]any{
		"name": "Spotify", "type": "expense", "amount": 99.5, "frequency": "month",
		"start_date": "2099-01-05", "category": "entertainment", "notes": "family",
	}, &out)
	if out.Name != "Spotify" || out.Amount != "99.50" || out.MonthlyAmount != "99.50" || out.Currency != "MXN" ||
		out.Account != "Card" || out.Category != "Entertainment" || out.Frequency != "month" || out.IntervalCount != 1 ||
		out.NextDue != "2099-01-05" || out.Status != "active" || out.Notes != "family" {
		t.Fatalf("unexpected output: %+v", out)
	}
	if !strings.Contains(text, "99.50 MXN every month") || !strings.Contains(text, "estimate") {
		t.Fatalf("unexpected text: %s", text)
	}

	// Installments and intervals.
	h.mustCall("create_recurring", map[string]any{
		"name": "Phone", "type": "expense", "amount": 1000, "frequency": "MONTH", "interval_count": 2,
		"total_payments": 6, "start_date": "2099-01-31",
	}, &out)
	if out.IntervalCount != 2 || out.TotalPayments != 6 || out.LastDue != "2099-11-30" || out.MonthlyAmount != "500.00" {
		t.Fatalf("installments: %+v", out)
	}
}

func TestCreateRecurringErrors(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	base := func(overrides map[string]any) map[string]any {
		args := map[string]any{"name": "Gym", "type": "expense", "amount": 10, "frequency": "month"}
		for k, v := range overrides {
			args[k] = v
		}
		return args
	}
	h.mustFail("create_recurring", base(nil), "no active accounts")

	h.account("Card", "MXN")
	h.account("Cash", "MXN")
	h.category("Salary", "income")
	h.mustFail("create_recurring", base(nil), "account is required because there are several accounts: Card (MXN), Cash (MXN)")
	h.mustFail("create_recurring", base(map[string]any{"account": "Nope"}), "available accounts")

	withAccount := func(overrides map[string]any) map[string]any {
		o := map[string]any{"account": "Card"}
		for k, v := range overrides {
			o[k] = v
		}
		return base(o)
	}
	h.mustFail("create_recurring", withAccount(map[string]any{"frequency": "daily"}), "week, month, year")
	h.mustFail("create_recurring", withAccount(map[string]any{"type": "transfer"}), "expense or income")
	h.mustFail("create_recurring", withAccount(map[string]any{"amount": 0}), "greater than zero")
	h.mustFail("create_recurring", withAccount(map[string]any{"amount": 1.234}), "too many decimals for MXN")
	h.mustFail("create_recurring", withAccount(map[string]any{"category": "Salary"}), "no expense category named")
	h.mustFail("create_recurring", withAccount(map[string]any{"start_date": "tomorrow"}), "start_on")
	h.mustFail("create_recurring", withAccount(map[string]any{"interval_count": 0}), "interval_count")

	h.mustCall("create_recurring", withAccount(map[string]any{"name": "Gym"}), nil)
	h.mustFail("create_recurring", withAccount(map[string]any{"name": "gym"}), `named "Gym" already exists`)
}

func TestListRecurring(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.account("Card", "MXN")
	var empty listRecurringOut
	text := h.mustCall("list_recurring", nil, &empty)
	if len(empty.Items) != 0 || len(empty.CommittedMonthly) != 0 || !strings.Contains(text, "No recurring items") {
		t.Fatalf("empty: %+v / %s", empty, text)
	}

	h.mustCall("create_recurring", map[string]any{"name": "Netflix", "type": "expense", "amount": 200, "frequency": "month", "start_date": "2099-01-01"}, nil)
	h.mustCall("create_recurring", map[string]any{"name": "Insurance", "type": "expense", "amount": 1200, "frequency": "year", "start_date": "2099-03-01"}, nil)
	h.mustCall("create_recurring", map[string]any{"name": "Salary", "type": "income", "amount": 5000, "frequency": "month", "start_date": "2099-01-15"}, nil)
	h.mustCall("create_recurring", map[string]any{"name": "Paused", "type": "expense", "amount": 50, "frequency": "month"}, nil)
	h.mustCall("update_recurring", map[string]any{"recurring": "Paused", "status": "paused"}, nil)
	h.mustCall("create_recurring", map[string]any{"name": "Old", "type": "expense", "amount": 70, "frequency": "month"}, nil)
	h.mustCall("update_recurring", map[string]any{"recurring": "Old", "status": "cancelled"}, nil)

	var out listRecurringOut
	text = h.mustCall("list_recurring", nil, &out)
	if len(out.Items) != 4 {
		t.Fatalf("cancelled should be hidden by default: %+v", out.Items)
	}
	if len(out.CommittedMonthly) != 1 {
		t.Fatalf("committed: %+v", out.CommittedMonthly)
	}
	c := out.CommittedMonthly[0]
	if c.Currency != "MXN" || c.Expense != "300.00" || c.Income != "5000.00" || c.Net != "4700.00" || c.ExpenseCount != 2 || c.IncomeCount != 1 {
		t.Fatalf("committed: %+v", c)
	}
	if !strings.Contains(text, "Netflix") || !strings.Contains(text, "next due 2099-01-01") || !strings.Contains(text, "Committed monthly cost") {
		t.Fatalf("text: %s", text)
	}

	var filtered listRecurringOut
	h.mustCall("list_recurring", map[string]any{"status": "cancelled"}, &filtered)
	if len(filtered.Items) != 1 || filtered.Items[0].Name != "Old" {
		t.Fatalf("cancelled filter: %+v", filtered.Items)
	}
	h.mustCall("list_recurring", map[string]any{"status": "all"}, &filtered)
	if len(filtered.Items) != 5 {
		t.Fatalf("all: %+v", filtered.Items)
	}
	h.mustCall("list_recurring", map[string]any{"type": "income"}, &filtered)
	if len(filtered.Items) != 1 || filtered.Items[0].Name != "Salary" {
		t.Fatalf("type filter: %+v", filtered.Items)
	}
	h.mustFail("list_recurring", map[string]any{"status": "stopped"}, "active, paused, cancelled, all")
	h.mustFail("list_recurring", map[string]any{"type": "transfer"}, "expense or income")
}

func TestUpdateRecurring(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.account("Card", "MXN")
	h.category("Entertainment", "expense")
	h.category("Leisure", "expense")
	h.mustCall("create_recurring", map[string]any{
		"name": "Netflix", "type": "expense", "amount": 200, "frequency": "month", "start_date": "2099-01-01",
		"category": "Entertainment", "total_payments": 12,
	}, nil)

	var out recurringOut
	h.mustCall("update_recurring", map[string]any{"recurring": "netflix", "amount": 219.99, "frequency": "year", "category": "Leisure"}, &out)
	if out.Amount != "219.99" || out.Frequency != "year" || out.Category != "Leisure" || out.MonthlyAmount != "18.33" {
		t.Fatalf("edit: %+v", out)
	}
	out = recurringOut{}
	h.mustCall("update_recurring", map[string]any{"recurring": "Netflix", "clear_category": true, "clear_total_payments": true, "name": "Netflix HD", "notes": "n"}, &out)
	if out.Category != "" || out.TotalPayments != 0 || out.Name != "Netflix HD" || out.Notes != "n" {
		t.Fatalf("clear: %+v", out)
	}

	// Partial name and id resolution, pause, cancel.
	out = recurringOut{}
	text := h.mustCall("update_recurring", map[string]any{"recurring": "flix", "status": "paused"}, &out)
	if out.Status != "paused" || out.NextDue != "" || !strings.Contains(text, "paused") {
		t.Fatalf("pause: %+v / %s", out, text)
	}
	h.mustCall("update_recurring", map[string]any{"recurring": out.ID, "status": "cancelled"}, &out)
	if out.Status != "cancelled" {
		t.Fatalf("cancel: %+v", out)
	}
	// A single cancelled item is still found by name, e.g. to reactivate it.
	h.mustCall("update_recurring", map[string]any{"recurring": "Netflix HD", "status": "active"}, &out)
	if out.Status != "active" {
		t.Fatalf("reactivate: %+v", out)
	}
}

func TestUpdateRecurringErrors(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.mustFail("update_recurring", map[string]any{"recurring": "X", "amount": 1}, "no recurring items")
	h.account("Card", "MXN")
	h.category("Salary", "income")
	h.mustCall("create_recurring", map[string]any{"name": "Netflix", "type": "expense", "amount": 200, "frequency": "month"}, nil)
	h.mustCall("create_recurring", map[string]any{"name": "Spotify", "type": "expense", "amount": 100, "frequency": "month"}, nil)

	h.mustFail("update_recurring", map[string]any{"recurring": "Nope", "amount": 1}, "available items: Netflix (active), Spotify (active)")
	h.mustFail("update_recurring", map[string]any{"recurring": "Netflix"}, "nothing to update")
	h.mustFail("update_recurring", map[string]any{"recurring": "Netflix", "name": "spotify"}, `named "Spotify" already exists`)
	h.mustFail("update_recurring", map[string]any{"recurring": "Netflix", "frequency": "daily"}, "week, month, year")
	h.mustFail("update_recurring", map[string]any{"recurring": "Netflix", "status": "stopped"}, "active, paused, cancelled")
	h.mustFail("update_recurring", map[string]any{"recurring": "Netflix", "amount": 1.234}, "too many decimals for MXN")
	h.mustFail("update_recurring", map[string]any{"recurring": "Netflix", "category": "Salary"}, "no expense category named")
	h.mustFail("update_recurring", map[string]any{"recurring": "Netflix", "category": "Salary", "clear_category": true}, "no expense category named")
	h.mustFail("update_recurring", map[string]any{"recurring": "Netflix", "total_payments": 3, "clear_total_payments": true}, "cannot be combined")
}

func TestRecurringNamesCanBeReusedAfterCancel(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.account("Card", "MXN")
	create := map[string]any{"name": "Disney+", "type": "expense", "amount": 150, "frequency": "month"}
	var first recurringOut
	h.mustCall("create_recurring", create, &first)
	h.mustCall("update_recurring", map[string]any{"recurring": "Disney+", "status": "cancelled"}, nil)

	// The cancelled name is free again; the open item wins when resolving.
	var second recurringOut
	h.mustCall("create_recurring", create, &second)
	if second.ID == first.ID {
		t.Fatal("expected a new item")
	}
	var out recurringOut
	h.mustCall("update_recurring", map[string]any{"recurring": "disney+", "amount": 160}, &out)
	if out.ID != second.ID {
		t.Fatalf("should resolve to the open item: %+v", out)
	}
	// The cancelled one cannot be reactivated while the name is taken.
	h.mustFail("update_recurring", map[string]any{"recurring": first.ID, "status": "active"}, `named "Disney+" already exists`)

	// Several cancelled items with the same name must be disambiguated by id.
	h.mustCall("update_recurring", map[string]any{"recurring": second.ID, "status": "cancelled"}, nil)
	h.mustFail("update_recurring", map[string]any{"recurring": "Disney+", "amount": 1}, "several cancelled recurring items")
}
