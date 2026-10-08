package mcpserver

import (
	"strings"
	"testing"
	"time"
)

func monthOffset(offset int) time.Time {
	now := time.Now().UTC()
	return time.Date(now.Year(), now.Month()+time.Month(offset), 15, 0, 0, 0, 0, time.UTC)
}

func currentMonth() string { return monthOffset(0).Format("2006-01") }

func (h *harness) setBudget(category, currency string, amount float64) {
	h.t.Helper()
	h.mustCall("set_budgets", map[string]any{"items": []map[string]any{{"category": category, "currency": currency, "amount": amount}}}, nil)
}

func findBudgetLine(t *testing.T, out budgetStatusOut, currency, category string) budgetLineOut {
	t.Helper()
	for _, c := range out.Currencies {
		if c.Currency != currency {
			continue
		}
		for _, l := range c.Categories {
			if l.Category == category {
				return l
			}
		}
	}
	t.Fatalf("no line %s/%s in %+v", currency, category, out)
	return budgetLineOut{}
}

func TestSetBudgetsAndStatus(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.account("Card", "MXN")
	h.account("Euro", "EUR")
	h.category("Food", "expense")
	h.category("Fun", "expense")
	h.addExpense(map[string]any{"amount": 40.25, "account": "Card", "category": "Food"})

	var set budgetStatusOut
	text := h.mustCall("set_budgets", map[string]any{"items": []map[string]any{
		{"category": "food", "currency": "mxn", "amount": 100.5},
		{"category": "Fun", "currency": "EUR", "amount": 0},
	}}, &set)
	if !strings.Contains(text, "Applied 2 budget changes") || set.Month != currentMonth() {
		t.Fatalf("unexpected result %q %+v", text, set)
	}
	food := findBudgetLine(t, set, "MXN", "Food")
	if food.Budget != "100.50" || food.Spent != "40.25" || food.Remaining != "60.25" || food.State != "ok" || food.BudgetSetIn != currentMonth() {
		t.Fatalf("unexpected food line: %+v", food)
	}
	if fun := findBudgetLine(t, set, "EUR", "Fun"); fun.Budget != "0.00" || fun.State != "ok" {
		t.Fatalf("zero budget should be kept: %+v", fun)
	}

	var status budgetStatusOut
	text = h.mustCall("get_budget_status", map[string]any{}, &status)
	if !strings.Contains(text, "Food: 40.25 of 100.50") || findBudgetLine(t, status, "MXN", "Food").Budget != "100.50" {
		t.Fatalf("status mismatch: %q", text)
	}

	h.mustCall("set_budgets", map[string]any{"items": []map[string]any{{"category": "Food", "currency": "MXN", "clear": true}}}, &set)
	if line := findBudgetLine(t, set, "MXN", "Food"); line.Budget != "" || line.State != "none" {
		t.Fatalf("budget should be cleared: %+v", line)
	}
}

func TestSetBudgetsFutureMonthInherits(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.category("Food", "expense")
	next := monthOffset(1).Format("2006-01")
	var out budgetStatusOut
	h.mustCall("set_budgets", map[string]any{"month": next, "items": []map[string]any{{"category": "Food", "currency": "MXN", "amount": 50}}}, &out)
	if out.Month != next || findBudgetLine(t, out, "MXN", "Food").Budget != "50.00" {
		t.Fatalf("unexpected: %+v", out)
	}
	h.mustCall("get_budget_status", map[string]any{"month": currentMonth()}, &out)
	if len(out.Currencies) != 0 {
		t.Fatalf("the current month must not inherit from a later one: %+v", out)
	}
}

func TestSetBudgetsErrors(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.category("Food", "expense")
	h.category("Salary", "income")
	item := func(m map[string]any) map[string]any { return map[string]any{"items": []map[string]any{m}} }

	h.mustFail("set_budgets", item(map[string]any{"category": "Travel", "currency": "MXN", "amount": 1}), "use one of: Food")
	h.mustFail("set_budgets", item(map[string]any{"category": "Salary", "currency": "MXN", "amount": 1}), "no expense category named")
	h.mustFail("set_budgets", item(map[string]any{"category": "Food", "currency": "MXN"}), "amount is required")
	h.mustFail("set_budgets", item(map[string]any{"category": "Food", "currency": "MXN", "amount": 1, "clear": true}), "cannot be combined")
	h.mustFail("set_budgets", item(map[string]any{"category": "Food", "currency": "MXN", "amount": -1}), "zero or greater")
	h.mustFail("set_budgets", item(map[string]any{"category": "Food", "currency": "MXN", "amount": 1.234}), "too many decimals")
	h.mustFail("set_budgets", item(map[string]any{"category": "Food", "currency": "ZZZ", "amount": 1}), "unsupported currency")
	h.mustFail("set_budgets", item(map[string]any{"category": "Food", "amount": 1}), "currency")
	h.mustFail("set_budgets", item(map[string]any{"category": "", "currency": "MXN", "amount": 1}), "category is required")
	h.mustFail("set_budgets", map[string]any{"month": "October", "items": []map[string]any{{"category": "Food", "currency": "MXN", "amount": 1}}}, "YYYY-MM")
	h.mustFail("set_budgets", map[string]any{"items": []map[string]any{}}, "at least one")
	h.mustFail("set_budgets", map[string]any{"items": []map[string]any{
		{"category": "Food", "currency": "MXN", "amount": 1},
		{"category": "Food", "currency": "MXN", "amount": 2},
	}}, "duplicates items[0]")
}

func TestSetBudgetsIsAtomic(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.category("Food", "expense")
	h.mustFail("set_budgets", map[string]any{"items": []map[string]any{
		{"category": "Food", "currency": "MXN", "amount": 10},
		{"category": "Nope", "currency": "MXN", "amount": 10},
	}}, "no budget was changed: items[1]")
	var out budgetStatusOut
	h.mustCall("get_budget_status", map[string]any{}, &out)
	if len(out.Currencies) != 0 {
		t.Fatalf("nothing should have been set: %+v", out)
	}
}

func TestGetBudgetStatusErrorsAndEmpty(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	text := h.mustCall("get_budget_status", map[string]any{}, nil)
	if !strings.Contains(text, "No budgets and no expenses") {
		t.Fatalf("unexpected empty text: %q", text)
	}
	h.mustFail("get_budget_status", map[string]any{"month": "2026-13"}, "YYYY-MM")
	h.mustFail("get_budget_status", map[string]any{"owner": "nobody"}, "no workspace member matches")
}

func TestGetBudgetStatusOwnerNarrowsSpending(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.account("Card", "MXN")
	h.category("Food", "expense")
	h.addExpense(map[string]any{"amount": 30, "account": "Card", "category": "Food"})
	h.setBudget("Food", "MXN", 100)

	var out budgetStatusOut
	h.mustCall("get_budget_status", map[string]any{"owner": "shared"}, &out)
	line := findBudgetLine(t, out, "MXN", "Food")
	if line.Spent != "30.00" || line.Budget != "100.00" {
		t.Fatalf("shared account spending expected: %+v", line)
	}
}

func TestSuggestBudgets(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.account("Card", "MXN")
	h.category("Food", "expense")
	for i, amount := range []float64{100, 200, 300} {
		h.addExpense(map[string]any{"amount": amount, "account": "Card", "category": "Food", "date": monthOffset(-3 + i).Format("2006-01-02")})
	}
	h.setBudget("Food", "MXN", 150)

	var out budgetSuggestionsOut
	text := h.mustCall("suggest_budgets", map[string]any{}, &out)
	if out.Month != currentMonth() || out.From != monthOffset(-3).Format("2006-01") || out.To != monthOffset(-1).Format("2006-01") {
		t.Fatalf("unexpected range: %+v", out)
	}
	if len(out.Currencies) != 1 || len(out.Currencies[0].Suggestions) != 1 {
		t.Fatalf("unexpected suggestions: %+v", out)
	}
	sg := out.Currencies[0].Suggestions[0]
	if sg.Category != "Food" || sg.Median != "200.00" || sg.Suggested != "200.00" || sg.CurrentBudget != "150.00" || len(sg.Months) != 3 || sg.Months[0].Spent != "100.00" {
		t.Fatalf("unexpected suggestion: %+v", sg)
	}
	if !strings.Contains(text, "nothing was changed") || !strings.Contains(text, "Food: suggested 200.00") {
		t.Fatalf("text should explain the suggestion: %q", text)
	}
}

func TestSuggestBudgetsEmptyAndErrors(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	text := h.mustCall("suggest_budgets", map[string]any{}, nil)
	if !strings.Contains(text, "No expense history") {
		t.Fatalf("unexpected text: %q", text)
	}
	h.mustFail("suggest_budgets", map[string]any{"month": "soon"}, "YYYY-MM")
}

func TestAddExpenseBudgetWarning(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.account("Card", "MXN")
	h.account("Euro", "EUR")
	h.category("Food", "expense")
	h.category("Fun", "expense")
	h.category("Salary", "income")
	h.setBudget("Food", "MXN", 100)

	var out transactionOut
	text := h.mustCall("add_expense", map[string]any{"amount": 50, "account": "Card", "category": "Food"}, &out)
	if out.BudgetWarning != nil || strings.Contains(text, "Budget warning") {
		t.Fatalf("no warning expected under 80%%: %q", text)
	}

	text = h.mustCall("add_expense", map[string]any{"amount": 35, "account": "Card", "category": "Food"}, &out)
	w := out.BudgetWarning
	if w == nil || w.State != "near" || w.Spent != "85.00" || w.Budget != "100.00" || w.Remaining != "15.00" || !strings.Contains(text, "Budget warning: Food is near its budget") {
		t.Fatalf("near warning expected: %q %+v", text, w)
	}

	text = h.mustCall("add_expense", map[string]any{"amount": 20, "account": "Card", "category": "Food"}, &out)
	if w := out.BudgetWarning; w == nil || w.State != "over" || w.Remaining != "-5.00" || !strings.Contains(text, "over its budget") {
		t.Fatalf("over warning expected: %q %+v", text, w)
	}

	// Other categories, currencies, uncategorized expenses and income never warn.
	for name, args := range map[string]map[string]any{
		"no budget":      {"amount": 500, "account": "Card", "category": "Fun"},
		"other currency": {"amount": 500, "account": "Euro", "category": "Food"},
		"uncategorized":  {"amount": 500, "account": "Card"},
	} {
		if text := h.mustCall("add_expense", args, &out); out.BudgetWarning != nil || strings.Contains(text, "Budget warning") {
			t.Errorf("%s: unexpected warning: %q", name, text)
		}
	}
	if text := h.mustCall("add_income", map[string]any{"amount": 500, "account": "Card", "category": "Salary"}, &out); out.BudgetWarning != nil || strings.Contains(text, "Budget warning") {
		t.Errorf("income: unexpected warning: %q", text)
	}
}

func TestAddExpenseBudgetWarningUsesExpenseMonth(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.account("Card", "MXN")
	h.category("Food", "expense")
	h.setBudget("Food", "MXN", 100)

	var out transactionOut
	// The budget starts this month, so last month has none.
	last := monthOffset(-1).Format("2006-01-02")
	text := h.mustCall("add_expense", map[string]any{"amount": 150, "account": "Card", "category": "Food", "date": last}, &out)
	if out.BudgetWarning != nil {
		t.Fatalf("last month has no budget: %q", text)
	}
	// A later month inherits it but starts from zero spending.
	next := monthOffset(1).Format("2006-01-02")
	h.mustCall("add_expense", map[string]any{"amount": 150, "account": "Card", "category": "Food", "date": next}, &out)
	if out.BudgetWarning == nil || out.BudgetWarning.Month != monthOffset(1).Format("2006-01") || out.BudgetWarning.Spent != "150.00" {
		t.Fatalf("the inherited budget should warn in the expense month: %+v", out.BudgetWarning)
	}
}

func TestAddTransactionsBudgetWarning(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.account("Card", "MXN")
	h.category("Food", "expense")
	h.category("Fun", "expense")
	h.setBudget("Food", "MXN", 100)
	h.setBudget("Fun", "MXN", 1000)

	var out transactionsOut
	text := h.mustCall("add_transactions", map[string]any{"items": []map[string]any{
		{"type": "expense", "amount": 60, "account": "Card", "category": "Food"},
		{"type": "expense", "amount": 60, "account": "Card", "category": "Food"},
		{"type": "expense", "amount": 10, "account": "Card", "category": "Fun"},
	}}, &out)
	if len(out.BudgetWarnings) != 1 || out.BudgetWarnings[0].Category != "Food" || out.BudgetWarnings[0].State != "over" || out.BudgetWarnings[0].Spent != "120.00" {
		t.Fatalf("one warning for Food expected: %+v", out.BudgetWarnings)
	}
	if strings.Count(text, "Budget warning") != 1 {
		t.Fatalf("text should carry the warning once: %q", text)
	}

	text = h.mustCall("add_transactions", map[string]any{"items": []map[string]any{
		{"type": "expense", "amount": 1, "account": "Card", "category": "Fun"},
	}}, &out)
	if len(out.BudgetWarnings) != 0 || strings.Contains(text, "Budget warning") {
		t.Fatalf("no warning expected: %q", text)
	}
}

func TestUpdateTransactionBudgetWarning(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.account("Card", "MXN")
	h.category("Food", "expense")
	h.category("Fun", "expense")
	h.setBudget("Food", "MXN", 100)
	tx := h.addExpense(map[string]any{"amount": 10, "account": "Card", "category": "Fun"})

	var out transactionOut
	text := h.mustCall("update_transaction", map[string]any{"id": tx.ID, "amount": 12}, &out)
	if out.BudgetWarning != nil || strings.Contains(text, "Budget warning") {
		t.Fatalf("no warning expected for an unbudgeted category: %q", text)
	}

	// Re-categorizing into a budget it overflows warns, counting the
	// expense once.
	text = h.mustCall("update_transaction", map[string]any{"id": tx.ID, "category": "Food", "amount": 120}, &out)
	if w := out.BudgetWarning; w == nil || w.State != "over" || w.Spent != "120.00" || !strings.Contains(text, "Budget warning: Food is over its budget") {
		t.Fatalf("over warning expected: %q %+v", text, w)
	}

	// Edits that do not move spending stay quiet.
	text = h.mustCall("update_transaction", map[string]any{"id": tx.ID, "description": "Dinner"}, &out)
	if out.BudgetWarning != nil || strings.Contains(text, "Budget warning") {
		t.Fatalf("a note edit should not warn: %q", text)
	}

	// Lowering the amount under 80% clears the warning.
	text = h.mustCall("update_transaction", map[string]any{"id": tx.ID, "amount": 50}, &out)
	if out.BudgetWarning != nil || strings.Contains(text, "Budget warning") {
		t.Fatalf("no warning expected under 80%%: %q", text)
	}
}

func TestUpdateTransactionsBudgetWarning(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.account("Card", "MXN")
	h.category("Food", "expense")
	h.category("Fun", "expense")
	h.setBudget("Food", "MXN", 100)
	a := h.addExpense(map[string]any{"amount": 70, "account": "Card", "category": "Fun"})
	b := h.addExpense(map[string]any{"amount": 70, "account": "Card", "category": "Fun"})

	var out transactionsOut
	text := h.mustCall("update_transactions", map[string]any{"items": []map[string]any{
		{"id": a.ID, "category": "Food"},
		{"id": b.ID, "category": "Food"},
	}}, &out)
	if len(out.BudgetWarnings) != 1 || out.BudgetWarnings[0].Spent != "140.00" || out.BudgetWarnings[0].State != "over" || !strings.Contains(text, "Budget warning") {
		t.Fatalf("one over warning expected: %q %+v", text, out.BudgetWarnings)
	}
}
