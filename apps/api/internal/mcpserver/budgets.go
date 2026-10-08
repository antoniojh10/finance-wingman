package mcpserver

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/finance"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/money"
)

// --- Tool inputs and outputs ---

type budgetStatusArgs struct {
	Month string `json:"month,omitempty" jsonschema:"Month as YYYY-MM. Defaults to the current month"`
	Owner string `json:"owner,omitempty" jsonschema:"Optional: \"me\", \"shared\" or a member name. Only narrows spent and committed to that person's accounts; budgets are workspace-wide"`
}

type budgetLineOut struct {
	Category    string `json:"category"`
	Budget      string `json:"budget,omitempty"`
	BudgetSetIn string `json:"budget_set_in,omitempty"`
	Spent       string `json:"spent"`
	Committed   string `json:"committed"`
	Remaining   string `json:"remaining,omitempty"`
	State       string `json:"state"`
}

type budgetCurrencyOut struct {
	Currency      string          `json:"currency"`
	Budgeted      string          `json:"budgeted"`
	Spent         string          `json:"spent"`
	Committed     string          `json:"committed"`
	Remaining     string          `json:"remaining"`
	Categories    []budgetLineOut `json:"categories"`
	Uncategorized *budgetLineOut  `json:"uncategorized,omitempty"`
}

type budgetStatusOut struct {
	Month      string              `json:"month"`
	Currencies []budgetCurrencyOut `json:"currencies"`
}

type budgetItemArgs struct {
	Category string   `json:"category" jsonschema:"Expense category name or ID (see list_categories)"`
	Currency string   `json:"currency" jsonschema:"ISO 4217 code of the budget, e.g. MXN. A category has an independent budget per currency"`
	Amount   *float64 `json:"amount,omitempty" jsonschema:"Monthly budget as a decimal in that currency, e.g. 3000.50 (0 means spend nothing). Later months inherit it. Omit when clear is true"`
	Clear    bool     `json:"clear,omitempty" jsonschema:"true to remove the budget from the month on (earlier months keep theirs). Do not combine with amount"`
}

type setBudgetsArgs struct {
	Month string           `json:"month,omitempty" jsonschema:"Month the budgets start in, as YYYY-MM. Defaults to the current month. Later months inherit the amount until one sets another"`
	Items []budgetItemArgs `json:"items" jsonschema:"Budgets to set or clear (1 to 100). All are applied or none are; each category and currency may appear once"`
}

type suggestBudgetsArgs struct {
	Month string `json:"month,omitempty" jsonschema:"Month to suggest budgets for, as YYYY-MM. Defaults to the current month"`
}

type suggestionMonthOut struct {
	Month string `json:"month"`
	Spent string `json:"spent"`
}

type budgetSuggestionOut struct {
	Category      string               `json:"category"`
	Months        []suggestionMonthOut `json:"months"`
	Median        string               `json:"median"`
	Recurring     string               `json:"recurring"`
	Suggested     string               `json:"suggested"`
	CurrentBudget string               `json:"current_budget,omitempty"`
}

type budgetSuggestionCurrencyOut struct {
	Currency    string                `json:"currency"`
	Suggestions []budgetSuggestionOut `json:"suggestions"`
}

type budgetSuggestionsOut struct {
	Month      string                        `json:"month"`
	From       string                        `json:"from"`
	To         string                        `json:"to"`
	Currencies []budgetSuggestionCurrencyOut `json:"currencies"`
}

// budgetWarningOut tells that an expense left its category near or over
// budget. Figures include the expense.
type budgetWarningOut struct {
	Category  string `json:"category"`
	Month     string `json:"month"`
	Currency  string `json:"currency"`
	State     string `json:"state"`
	Budget    string `json:"budget"`
	Spent     string `json:"spent"`
	Committed string `json:"committed"`
	Remaining string `json:"remaining"`
}

func (w budgetWarningOut) message() string {
	state := "near its budget"
	if w.State == finance.BudgetOver {
		state = "over its budget"
	}
	return fmt.Sprintf("Budget warning: %s is %s for %s: %s spent (plus %s committed in recurring expenses) of %s %s, remaining %s.",
		w.Category, state, w.Month, w.Spent, w.Committed, w.Budget, w.Currency, w.Remaining)
}

func (s *Server) registerBudgetTools() {
	addTool(s, &mcp.Tool{
		Name:  "get_budget_status",
		Title: "Budget status",
		Description: "Monthly budgets per currency and expense category: the budget in force (set in the month or inherited from an earlier one), spent, committed (unpaid recurring expenses due in the month, not counted as spent), remaining and state (ok, near at 80% or more, over, none when the category has no budget). " +
			"Also lists categories with spending but no budget and uncategorized expenses. Amounts stay in each currency; never add currencies together.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: boolPtr(false)},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args budgetStatusArgs) (*mcp.CallToolResult, budgetStatusOut, error) {
		return s.budgetStatus(ctx, args)
	})

	addTool(s, &mcp.Tool{
		Name:  "set_budgets",
		Title: "Set budgets",
		Description: "Set or clear monthly budgets for expense categories, one amount per category and currency, starting in a month (later months inherit it). " +
			"All items are applied or none are. Confirm the amounts with the user before calling it, and run suggest_budgets first when they do not know what to set. Returns the resulting budget status.",
		Annotations: &mcp.ToolAnnotations{DestructiveHint: boolPtr(false), IdempotentHint: true, OpenWorldHint: boolPtr(false)},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args setBudgetsArgs) (*mcp.CallToolResult, budgetStatusOut, error) {
		return s.setBudgets(ctx, args)
	})

	addTool(s, &mcp.Tool{
		Name:  "suggest_budgets",
		Title: "Suggest budgets",
		Description: "Suggest a monthly budget per currency and expense category from the 3 complete months before the month: the median monthly spending plus unpaid recurring expenses, rounded up to a whole currency unit. " +
			"Each suggestion lists the months used so you can explain it. It changes nothing: present the suggestions to the user and call set_budgets only after they agree.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: boolPtr(false)},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args suggestBudgetsArgs) (*mcp.CallToolResult, budgetSuggestionsOut, error) {
		return s.suggestBudgets(ctx, args)
	})
}

// --- Handlers ---

func (s *Server) budgetStatus(ctx context.Context, args budgetStatusArgs) (*mcp.CallToolResult, budgetStatusOut, error) {
	owner, err := s.ownerFilter(ctx, args.Owner)
	if err != nil {
		return nil, budgetStatusOut{}, err
	}
	status, err := s.finance.BudgetStatus(ctx, strings.TrimSpace(args.Month), owner)
	if err != nil {
		return nil, budgetStatusOut{}, friendly(err)
	}
	return text(renderBudgetStatus("Budgets", status)), budgetStatusToOut(status), nil
}

func (s *Server) setBudgets(ctx context.Context, args setBudgetsArgs) (*mcp.CallToolResult, budgetStatusOut, error) {
	if err := checkBatch(len(args.Items)); err != nil {
		return nil, budgetStatusOut{}, err
	}
	month := strings.TrimSpace(args.Month)
	if month == "" {
		month = s.finance.Today().Format("2006-01")
	}
	currencies := map[string]finance.Currency{}
	items := make([]finance.BudgetItemInput, len(args.Items))
	for i, item := range args.Items {
		in, err := s.budgetItemInput(ctx, item, currencies)
		if err != nil {
			return nil, budgetStatusOut{}, budgetFailure(itemErr(i, "", err))
		}
		items[i] = in
	}
	status, err := s.finance.SetBudgets(ctx, month, items)
	if err != nil {
		return nil, budgetStatusOut{}, budgetFailure(err)
	}
	msg := fmt.Sprintf("Applied %d budget changes starting in %s.\n%s", len(items), status.Month, renderBudgetStatus("Resulting budgets", status))
	return text(msg), budgetStatusToOut(status), nil
}

// budgetItemInput resolves the category and converts the decimal amount of
// one set_budgets item.
func (s *Server) budgetItemInput(ctx context.Context, item budgetItemArgs, currencies map[string]finance.Currency) (finance.BudgetItemInput, error) {
	var in finance.BudgetItemInput
	if strings.TrimSpace(item.Category) == "" {
		return in, fmt.Errorf("category is required: pass an expense category name (see list_categories)")
	}
	if strings.TrimSpace(item.Currency) == "" {
		return in, fmt.Errorf("currency is required: pass the ISO 4217 code of the budget, e.g. MXN (see list_accounts for the currencies in use)")
	}
	category, err := s.resolveCategory(ctx, item.Category, finance.TypeExpense)
	if err != nil {
		return in, err
	}
	code := strings.ToUpper(strings.TrimSpace(item.Currency))
	currency, ok := currencies[code]
	if !ok {
		if currency, err = s.findCurrency(ctx, code); err != nil {
			return in, err
		}
		currencies[code] = currency
	}
	in = finance.BudgetItemInput{CategoryID: category.ID, Currency: currency.Code, Clear: item.Clear}
	switch {
	case item.Clear && item.Amount != nil:
		return in, fmt.Errorf("clear cannot be combined with amount: pass one of them")
	case !item.Clear && item.Amount == nil:
		return in, fmt.Errorf("amount is required unless clear is true")
	case item.Amount != nil && *item.Amount < 0:
		return in, fmt.Errorf("amount must be zero or greater")
	case item.Amount != nil:
		amount, err := decimalToMinor(*item.Amount, currency.MinorUnits, currency.Code, "amount")
		if err != nil {
			return in, err
		}
		in.Amount = &amount
	}
	return in, nil
}

// budgetFailure explains that a failed set_budgets changed nothing.
func budgetFailure(err error) error {
	return fmt.Errorf("no budget was changed: %s. Fix that item and resend the whole call", friendly(err).Error())
}

func (s *Server) suggestBudgets(ctx context.Context, args suggestBudgetsArgs) (*mcp.CallToolResult, budgetSuggestionsOut, error) {
	res, err := s.finance.SuggestBudgets(ctx, strings.TrimSpace(args.Month))
	if err != nil {
		return nil, budgetSuggestionsOut{}, friendly(err)
	}
	out := budgetSuggestionsOut{Month: res.Month, From: res.From, To: res.To, Currencies: make([]budgetSuggestionCurrencyOut, len(res.Currencies))}
	var b strings.Builder
	if len(res.Currencies) == 0 {
		fmt.Fprintf(&b, "No expense history from %s to %s to base budgets on for %s. Ask the user for amounts and use set_budgets.", res.From, res.To, res.Month)
		return text(b.String()), out, nil
	}
	fmt.Fprintf(&b, "Suggested budgets for %s based on %s to %s (nothing was changed):", res.Month, res.From, res.To)
	for i, cur := range res.Currencies {
		oc := budgetSuggestionCurrencyOut{Currency: cur.Currency, Suggestions: make([]budgetSuggestionOut, len(cur.Suggestions))}
		fmt.Fprintf(&b, "\n%s:", cur.Currency)
		for j, sg := range cur.Suggestions {
			os := budgetSuggestionOut{
				Category:  sg.CategoryName,
				Months:    make([]suggestionMonthOut, len(sg.Months)),
				Median:    money.Format(sg.Median, cur.MinorUnits),
				Recurring: money.Format(sg.Recurring, cur.MinorUnits),
				Suggested: money.Format(sg.Suggested, cur.MinorUnits),
			}
			spent := make([]string, len(sg.Months))
			for k, m := range sg.Months {
				os.Months[k] = suggestionMonthOut{Month: m.Month, Spent: money.Format(m.Spent, cur.MinorUnits)}
				spent[k] = m.Month + " " + os.Months[k].Spent
			}
			current := ""
			if sg.CurrentAmount != nil {
				os.CurrentBudget = money.Format(*sg.CurrentAmount, cur.MinorUnits)
				current = ", current budget " + os.CurrentBudget
			}
			recurring := ""
			if sg.Recurring > 0 {
				recurring = " + " + os.Recurring + " recurring"
			}
			fmt.Fprintf(&b, "\n- %s: suggested %s (median %s%s over %s%s)", sg.CategoryName, os.Suggested, os.Median, recurring, strings.Join(spent, ", "), current)
			oc.Suggestions[j] = os
		}
		out.Currencies[i] = oc
	}
	return text(b.String()), out, nil
}

// --- Rendering ---

func budgetLineToOut(l finance.BudgetLine, units int) budgetLineOut {
	out := budgetLineOut{
		Category:  categoryLabel(l.CategoryName),
		Spent:     money.Format(l.Spent, units),
		Committed: money.Format(l.Committed, units),
		State:     l.State,
	}
	if l.Amount != nil {
		out.Budget = money.Format(*l.Amount, units)
	}
	if l.AmountMonth != nil {
		out.BudgetSetIn = *l.AmountMonth
	}
	if l.Remaining != nil {
		out.Remaining = money.Format(*l.Remaining, units)
	}
	return out
}

func budgetStatusToOut(status finance.BudgetStatus) budgetStatusOut {
	out := budgetStatusOut{Month: status.Month, Currencies: make([]budgetCurrencyOut, len(status.Currencies))}
	for i, cur := range status.Currencies {
		oc := budgetCurrencyOut{
			Currency:   cur.Currency,
			Budgeted:   money.Format(cur.Budgeted, cur.MinorUnits),
			Spent:      money.Format(cur.Spent, cur.MinorUnits),
			Committed:  money.Format(cur.Committed, cur.MinorUnits),
			Remaining:  money.Format(cur.Remaining, cur.MinorUnits),
			Categories: make([]budgetLineOut, len(cur.Categories)),
		}
		for j, l := range cur.Categories {
			oc.Categories[j] = budgetLineToOut(l, cur.MinorUnits)
		}
		if cur.Uncategorized != nil {
			u := budgetLineToOut(*cur.Uncategorized, cur.MinorUnits)
			oc.Uncategorized = &u
		}
		out.Currencies[i] = oc
	}
	return out
}

func renderBudgetStatus(title string, status finance.BudgetStatus) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s for %s:", title, status.Month)
	out := budgetStatusToOut(status)
	if len(out.Currencies) == 0 {
		b.WriteString("\nNo budgets and no expenses this month.")
		return b.String()
	}
	for _, cur := range out.Currencies {
		fmt.Fprintf(&b, "\n%s: budgeted %s, spent %s, committed %s, remaining %s", cur.Currency, cur.Budgeted, cur.Spent, cur.Committed, cur.Remaining)
		lines := cur.Categories
		if cur.Uncategorized != nil {
			lines = append(lines, *cur.Uncategorized)
		}
		for _, l := range lines {
			if l.Budget == "" {
				fmt.Fprintf(&b, "\n- %s: no budget, spent %s, committed %s", l.Category, l.Spent, l.Committed)
				continue
			}
			fmt.Fprintf(&b, "\n- %s: %s of %s (committed %s, remaining %s) [%s]", l.Category, l.Spent, l.Budget, l.Committed, l.Remaining, l.State)
		}
	}
	return b.String()
}

// --- Over-budget warnings ---

// budgetWarnings reports, for the expenses among txs that are already
// recorded, the categories that are near or over budget in the month of the
// expense. Each category, currency and month is reported once. The check is
// best effort: a failure to evaluate it never fails the tool call.
func (s *Server) budgetWarnings(ctx context.Context, txs []finance.Transaction) []budgetWarningOut {
	type key struct{ category, currency, month string }
	seen := map[key]bool{}
	var out []budgetWarningOut
	for _, tx := range txs {
		if tx.Type != finance.TypeExpense || tx.CategoryID == nil {
			continue
		}
		date, err := time.Parse(time.DateOnly, tx.OccurredOn)
		if err != nil {
			continue
		}
		k := key{tx.CategoryID.String(), tx.Currency, date.Format("2006-01")}
		if seen[k] {
			continue
		}
		seen[k] = true
		// The expense is already recorded, so adding zero reads the state
		// it left the budget in without counting it twice.
		impact, err := s.finance.BudgetImpact(ctx, *tx.CategoryID, tx.Currency, date, 0)
		if err != nil || !impact.HasBudget || !impact.Warning {
			continue
		}
		units := tx.MinorUnits
		out = append(out, budgetWarningOut{
			Category:  categoryLabel(tx.CategoryName),
			Month:     impact.Month,
			Currency:  impact.Currency,
			State:     impact.StateAfter,
			Budget:    money.Format(impact.Amount, units),
			Spent:     money.Format(impact.Spent, units),
			Committed: money.Format(impact.Committed, units),
			Remaining: money.Format(impact.Remaining, units),
		})
	}
	return out
}

func warningText(warnings []budgetWarningOut) string {
	var b strings.Builder
	for _, w := range warnings {
		b.WriteString("\n" + w.message())
	}
	return b.String()
}
