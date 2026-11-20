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

type balanceHistoryArgs struct {
	From      string `json:"from,omitempty" jsonschema:"Optional start date, YYYY-MM-DD. Defaults to the first day of the current month"`
	To        string `json:"to,omitempty" jsonschema:"Optional end date, YYYY-MM-DD. Defaults to the last day of the current month. Days after today are projected. The range spans at most 92 days"`
	Owner     string `json:"owner,omitempty" jsonschema:"Optional: \"me\", \"shared\" or a member name. Only counts that person's accounts"`
	ByAccount bool   `json:"by_account,omitempty" jsonschema:"Also return the daily balance of every account. Defaults to false"`
}

type balanceDayOut struct {
	Date              string `json:"date"`
	Balance           string `json:"balance" jsonschema:"End-of-day balance of accounts other than savings and investment"`
	SavingsBalance    string `json:"savings_balance"`
	InvestmentBalance string `json:"investment_balance"`
	Income            string `json:"income" jsonschema:"Income recorded that day, transfers excluded"`
	Expense           string `json:"expense" jsonschema:"Expenses recorded that day, transfers excluded"`
}

type balancePointOut struct {
	Date    string `json:"date"`
	Balance string `json:"balance"`
}

type balanceProjectedRecurringOut struct {
	Name      string `json:"name"`
	DueOn     string `json:"due_on"`
	AppliedOn string `json:"applied_on"`
	Amount    string `json:"amount" jsonschema:"Signed: income positive, expense negative"`
}

type balanceProjectionBasisOut struct {
	AverageDailyExpense string                         `json:"average_daily_expense" jsonschema:"Average daily spending not linked to recurring items over the last 30 days"`
	Recurring           []balanceProjectedRecurringOut `json:"recurring" jsonschema:"Unpaid recurring occurrences applied to the projection"`
}

type balanceHeaderOut struct {
	StartBalance        string  `json:"start_balance" jsonschema:"Balance at the end of the first day of the range"`
	EndBalance          string  `json:"end_balance" jsonschema:"Balance at the end of the last recorded day (today, or the end of the range when it is in the past)"`
	EndDate             string  `json:"end_date"`
	MinBalance          string  `json:"min_balance" jsonschema:"Lowest balance in the range, projection included"`
	MinDate             string  `json:"min_date"`
	MaxBalance          string  `json:"max_balance" jsonschema:"Highest balance in the range, projection included"`
	MaxDate             string  `json:"max_date"`
	ProjectedEndBalance *string `json:"projected_end_balance" jsonschema:"Projected balance on the last day of the range; null when the range does not go past today"`
}

type balanceCurrencyOut struct {
	Currency        string                    `json:"currency"`
	Summary         *balanceHeaderOut         `json:"summary" jsonschema:"Computed highlights; null when the range has no recorded days"`
	Days            []balanceDayOut           `json:"days" jsonschema:"One entry per day from the start of the range up to today (or the end of the range)"`
	Projection      []balancePointOut         `json:"projection" jsonschema:"Projected balance for the days after today; empty when the range does not go past today"`
	ProjectionBasis balanceProjectionBasisOut `json:"projection_basis"`
}

type balanceAccountOut struct {
	Name     string            `json:"name"`
	Type     string            `json:"type"`
	Currency string            `json:"currency"`
	Days     []balancePointOut `json:"days"`
}

type balanceHistoryOut struct {
	From       string               `json:"from"`
	To         string               `json:"to"`
	Today      string               `json:"today"`
	Currencies []balanceCurrencyOut `json:"currencies"`
	Accounts   []balanceAccountOut  `json:"accounts,omitempty" jsonschema:"Only with by_account"`
}

func (s *Server) registerBalanceHistoryTools() {
	addTool(s, &mcp.Tool{
		Name:  "get_balance_history",
		Title: "Balance history",
		Description: "Day-by-day balance per currency over a date range (default: the current month, at most 92 days), with income and expenses per day, " +
			"highlights (start, end, lowest and highest balance with dates) and, when the range goes past today, a projection of the balance up to its end. " +
			"Use it for \"how did my balance evolve this month?\" or \"will I make it to the end of the month?\". " +
			"The balance excludes savings and investment accounts, which are reported separately. " +
			"The projection starts from today's balance and, for each later day, subtracts the average daily spending of the last 30 days (spending not linked to recurring items), " +
			"applies the unpaid recurring items due that day (those already due and unpaid land tomorrow) and the transactions already recorded for it. It is an estimate, not a forecast of unknown income. " +
			"Amounts stay in each currency: currencies are never converted; never add them together. Set by_account to also get each account's daily balance.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: boolPtr(false)},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args balanceHistoryArgs) (*mcp.CallToolResult, balanceHistoryOut, error) {
		return s.balanceHistory(ctx, args)
	})
}

// --- Handlers ---

func (s *Server) balanceHistory(ctx context.Context, args balanceHistoryArgs) (*mcp.CallToolResult, balanceHistoryOut, error) {
	owner, err := s.ownerFilter(ctx, args.Owner)
	if err != nil {
		return nil, balanceHistoryOut{}, err
	}
	sum, err := s.finance.DailyBalances(ctx, strings.TrimSpace(args.From), strings.TrimSpace(args.To), owner)
	if err != nil {
		err = friendly(err)
		if strings.Contains(err.Error(), "92 days") {
			err = fmt.Errorf("%w; use a range of at most 92 days", err)
		}
		return nil, balanceHistoryOut{}, err
	}

	out := balanceHistoryOut{From: sum.From, To: sum.To, Today: sum.Today, Currencies: make([]balanceCurrencyOut, len(sum.Currencies))}
	var b strings.Builder
	fmt.Fprintf(&b, "Balance history from %s to %s (today is %s):", sum.From, sum.To, sum.Today)
	if len(sum.Currencies) == 0 {
		b.WriteString("\nNo accounts to show.")
	}
	for i, c := range sum.Currencies {
		co := balanceCurrencyToOut(c)
		out.Currencies[i] = co
		fmt.Fprintf(&b, "\n\n%s:", c.Currency)
		if h := co.Summary; h != nil {
			fmt.Fprintf(&b, "\n- Start %s, end %s (%s), lowest %s (%s), highest %s (%s)", h.StartBalance, h.EndBalance, h.EndDate, h.MinBalance, h.MinDate, h.MaxBalance, h.MaxDate)
			if h.ProjectedEndBalance != nil {
				fmt.Fprintf(&b, "\n- Projected balance on %s: %s", sum.To, *h.ProjectedEndBalance)
			}
		}
		if len(co.Projection) > 0 {
			fmt.Fprintf(&b, "\n- Projection basis: average daily expense %s; %d unpaid recurring occurrences applied", co.ProjectionBasis.AverageDailyExpense, len(co.ProjectionBasis.Recurring))
		}
	}
	if args.ByAccount {
		out.Accounts = make([]balanceAccountOut, len(sum.Accounts))
		for i, a := range sum.Accounts {
			days := make([]balancePointOut, len(a.Days))
			for j, d := range a.Days {
				days[j] = balancePointOut{Date: d.Date, Balance: money.Format(d.Balance, a.MinorUnits)}
			}
			out.Accounts[i] = balanceAccountOut{Name: a.Name, Type: a.Type, Currency: a.Currency, Days: days}
		}
	}
	return text(b.String()), out, nil
}

func balanceCurrencyToOut(c finance.DailyCurrency) balanceCurrencyOut {
	f := func(v int64) string { return money.Format(v, c.MinorUnits) }
	out := balanceCurrencyOut{
		Currency:   c.Currency,
		Days:       make([]balanceDayOut, len(c.Days)),
		Projection: make([]balancePointOut, len(c.Projection)),
		ProjectionBasis: balanceProjectionBasisOut{
			AverageDailyExpense: f(c.ProjectionBasis.AverageDailyExpense),
			Recurring:           make([]balanceProjectedRecurringOut, len(c.ProjectionBasis.Recurring)),
		},
	}
	for i, d := range c.Days {
		out.Days[i] = balanceDayOut{Date: d.Date, Balance: f(d.Balance), SavingsBalance: f(d.SavingsBalance), InvestmentBalance: f(d.InvestmentBalance), Income: f(d.Income), Expense: f(d.Expense)}
	}
	for i, p := range c.Projection {
		out.Projection[i] = balancePointOut{Date: p.Date, Balance: f(p.Balance)}
	}
	for i, r := range c.ProjectionBasis.Recurring {
		out.ProjectionBasis.Recurring[i] = balanceProjectedRecurringOut{Name: r.Name, DueOn: r.DueOn, AppliedOn: r.AppliedOn, Amount: f(r.Amount)}
	}

	if len(c.Days) == 0 {
		return out
	}
	last := c.Days[len(c.Days)-1]
	h := &balanceHeaderOut{StartBalance: f(c.Days[0].Balance), EndBalance: f(last.Balance), EndDate: last.Date}
	type point struct {
		date    string
		balance int64
	}
	points := make([]point, 0, len(c.Days)+len(c.Projection))
	for _, d := range c.Days {
		points = append(points, point{d.Date, d.Balance})
	}
	for _, p := range c.Projection {
		points = append(points, point{p.Date, p.Balance})
	}
	lo, hi := points[0], points[0]
	for _, p := range points {
		if p.balance < lo.balance {
			lo = p
		}
		if p.balance > hi.balance {
			hi = p
		}
	}
	h.MinBalance, h.MinDate, h.MaxBalance, h.MaxDate = f(lo.balance), lo.date, f(hi.balance), hi.date
	if n := len(c.Projection); n > 0 {
		v := f(c.Projection[n-1].Balance)
		h.ProjectedEndBalance = &v
	}
	out.Summary = h
	return out
}
