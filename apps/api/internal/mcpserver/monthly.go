package mcpserver

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/finance"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/money"
)

// --- Tool inputs and outputs ---

type monthlySpendingArgs struct {
	Months   int    `json:"months,omitempty" jsonschema:"Number of calendar months ending with the current one: 3, 6 or 12. Defaults to 6"`
	Owner    string `json:"owner,omitempty" jsonschema:"Optional: \"me\", \"shared\" or a member name. Only counts that person's accounts; budgets are workspace-wide and are not returned with an owner"`
	Currency string `json:"currency,omitempty" jsonschema:"Optional ISO 4217 code, e.g. MXN. Returns only that currency"`
}

type monthlyCategoryOut struct {
	Category string    `json:"category"`
	Archived bool      `json:"archived"`
	Totals   []string  `json:"totals" jsonschema:"Expenses per month, same order as months"`
	Budgets  []*string `json:"budgets" jsonschema:"Budget in force per month; null when none (or with an owner filter)"`
}

type monthlyCurrencyOut struct {
	Currency     string               `json:"currency"`
	PartialMonth string               `json:"partial_month" jsonschema:"The current month, still in progress"`
	Totals       []string             `json:"totals"`
	Budgets      []*string            `json:"budgets" jsonschema:"Sum of the budgets in force per month; null when none is set (or with an owner filter)"`
	Categories   []monthlyCategoryOut `json:"categories" jsonschema:"Largest period total first; uncategorized expenses are named Uncategorized"`
}

type monthlySpendingOut struct {
	Months     []string             `json:"months" jsonschema:"Calendar months (YYYY-MM), oldest first"`
	Currencies []monthlyCurrencyOut `json:"currencies"`
}

func (s *Server) registerMonthlyTools() {
	addTool(s, &mcp.Tool{
		Name:  "get_monthly_spending",
		Title: "Monthly spending",
		Description: "Expenses per month and category over the last 3, 6 or 12 calendar months (the current month included, still in progress), per currency, with the budget in force each month. " +
			"Use it to compare periods, e.g. which categories grew compared to an earlier season. Transfers and income are not counted. Amounts stay in each currency; never add currencies together.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: boolPtr(false)},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args monthlySpendingArgs) (*mcp.CallToolResult, monthlySpendingOut, error) {
		return s.monthlySpending(ctx, args)
	})
}

// --- Handlers ---

func (s *Server) monthlySpending(ctx context.Context, args monthlySpendingArgs) (*mcp.CallToolResult, monthlySpendingOut, error) {
	months := args.Months
	if months == 0 {
		months = 6
	}
	if months != 3 && months != 6 && months != 12 {
		return nil, monthlySpendingOut{}, fmt.Errorf("months must be 3, 6 or 12, got %d", args.Months)
	}
	currency := strings.ToUpper(strings.TrimSpace(args.Currency))
	if currency != "" {
		if _, err := s.findCurrency(ctx, currency); err != nil {
			return nil, monthlySpendingOut{}, err
		}
	}
	owner, err := s.ownerFilter(ctx, args.Owner)
	if err != nil {
		return nil, monthlySpendingOut{}, err
	}
	sum, err := s.finance.MonthlySpending(ctx, months, owner)
	if err != nil {
		return nil, monthlySpendingOut{}, friendly(err)
	}

	out := monthlySpendingOut{Months: sum.Months, Currencies: []monthlyCurrencyOut{}}
	present := make([]string, 0, len(sum.Currencies))
	for _, c := range sum.Currencies {
		present = append(present, c.Currency)
		if currency != "" && c.Currency != currency {
			continue
		}
		out.Currencies = append(out.Currencies, monthlyCurrencyToOut(c))
	}
	if currency != "" && len(out.Currencies) == 0 {
		if len(present) == 0 {
			return nil, monthlySpendingOut{}, fmt.Errorf("no expenses in %s over the last %d months and the workspace has no expenses in that period; omit currency to see all", currency, months)
		}
		return nil, monthlySpendingOut{}, fmt.Errorf("no expenses in %s over the last %d months; currencies with expenses in that period: %s", currency, months, strings.Join(present, ", "))
	}
	return text(renderMonthlySpending(out, months, owner.UserID != nil || owner.Shared)), out, nil
}

// --- Rendering ---

func monthlyCurrencyToOut(c finance.MonthlyCurrency) monthlyCurrencyOut {
	f := func(v int64) string { return money.Format(v, c.MinorUnits) }
	out := monthlyCurrencyOut{
		Currency:     c.Currency,
		PartialMonth: c.PartialMonth,
		Totals:       make([]string, len(c.Totals)),
		Budgets:      monthlyAmounts(c.Budgets, f),
		Categories:   make([]monthlyCategoryOut, len(c.Categories)),
	}
	for i, v := range c.Totals {
		out.Totals[i] = f(v)
	}
	for i, cat := range c.Categories {
		oc := monthlyCategoryOut{
			Category: categoryLabel(cat.Name),
			Archived: cat.Archived,
			Totals:   make([]string, len(cat.Totals)),
			Budgets:  monthlyAmounts(cat.Budgets, f),
		}
		for j, v := range cat.Totals {
			oc.Totals[j] = f(v)
		}
		out.Categories[i] = oc
	}
	return out
}

func monthlyAmounts(values []*int64, format func(int64) string) []*string {
	out := make([]*string, len(values))
	for i, v := range values {
		if v != nil {
			s := format(*v)
			out[i] = &s
		}
	}
	return out
}

func renderMonthlySpending(out monthlySpendingOut, months int, owned bool) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Expenses by category, last %d months (%s is still in progress):", months, out.Months[len(out.Months)-1])
	if len(out.Currencies) == 0 {
		b.WriteString("\nNo expenses in this period.")
		return b.String()
	}
	for _, c := range out.Currencies {
		fmt.Fprintf(&b, "\n\n%s (months: %s)", c.Currency, strings.Join(out.Months, ", "))
		for _, cat := range c.Categories {
			fmt.Fprintf(&b, "\n- %s: %s", cat.Category, strings.Join(cat.Totals, ", "))
			if cat.Archived {
				b.WriteString(" (archived)")
			}
		}
		fmt.Fprintf(&b, "\nTotal: %s", strings.Join(c.Totals, ", "))
		if !owned {
			fmt.Fprintf(&b, "\nBudget: %s", strings.Join(valuesOrDash(c.Budgets), ", "))
		}
	}
	return b.String()
}

// valuesOrDash renders nullable amounts, with "-" for months without a value.
func valuesOrDash(values []*string) []string {
	out := make([]string, len(values))
	for i, v := range values {
		if v == nil {
			out[i] = "-"
		} else {
			out[i] = *v
		}
	}
	return out
}
