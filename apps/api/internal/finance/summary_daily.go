package finance

import (
	"context"
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/store"
)

const (
	maxDailySpanDays      = 92
	averageSpendingDays   = 30
	accountTypeSavings    = "savings"
	accountTypeInvestment = "investment"
)

// DailyBalance is the end-of-day state of a currency.
type DailyBalance struct {
	Date              string `json:"date" format:"date"`
	Balance           int64  `json:"balance" doc:"End-of-day balance of accounts other than savings and investment, in minor units"`
	SavingsBalance    int64  `json:"savings_balance" doc:"End-of-day balance of savings accounts, in minor units"`
	InvestmentBalance int64  `json:"investment_balance" doc:"End-of-day balance of investment accounts, in minor units"`
	Income            int64  `json:"income" doc:"Income recorded that day (transfers excluded), in minor units"`
	Expense           int64  `json:"expense" doc:"Expenses recorded that day (transfers excluded), in minor units"`
}

// ProjectedBalance is the projected end-of-day balance.
type ProjectedBalance struct {
	Date    string `json:"date" format:"date"`
	Balance int64  `json:"balance" doc:"Projected balance of accounts other than savings and investment, in minor units"`
}

// ProjectedRecurring is an unpaid recurring occurrence included in the projection.
type ProjectedRecurring struct {
	RecurringID uuid.UUID `json:"recurring_id"`
	Name        string    `json:"name"`
	DueOn       string    `json:"due_on" format:"date" doc:"Real due date of the occurrence"`
	AppliedOn   string    `json:"applied_on" format:"date" doc:"Day it is applied in the projection: the due date, or tomorrow when it is already due and unpaid"`
	Amount      int64     `json:"amount" doc:"Signed amount in minor units: income positive, expense negative"`
}

// ProjectionBasis explains how a projection was computed.
type ProjectionBasis struct {
	AverageDailyExpense int64                `json:"average_daily_expense" doc:"Average daily spending not linked to recurring items over the last 30 days, in minor units"`
	Recurring           []ProjectedRecurring `json:"recurring" doc:"Unpaid recurring occurrences applied to the projection"`
}

// DailyCurrency is the day-by-day evolution of one currency.
type DailyCurrency struct {
	Currency        string             `json:"currency" example:"MXN"`
	MinorUnits      int                `json:"minor_units" example:"2"`
	Days            []DailyBalance     `json:"days" doc:"One entry per day from the start of the range up to today (or the end of the range)"`
	Projection      []ProjectedBalance `json:"projection" doc:"Projected balance for the days after today; empty when the range does not go past today"`
	ProjectionBasis ProjectionBasis    `json:"projection_basis"`
}

// AccountBalance is the end-of-day balance of an account.
type AccountBalance struct {
	Date    string `json:"date" format:"date"`
	Balance int64  `json:"balance" doc:"End-of-day balance in minor units"`
}

// DailyAccount is the day-by-day balance of one account.
type DailyAccount struct {
	AccountID  uuid.UUID        `json:"account_id"`
	Name       string           `json:"name"`
	Type       string           `json:"type" enum:"checking,savings,credit_card,cash,investment,other"`
	Currency   string           `json:"currency" example:"MXN"`
	MinorUnits int              `json:"minor_units" example:"2"`
	Days       []AccountBalance `json:"days"`
}

// DailySummary is the daily balance evolution of a period.
type DailySummary struct {
	From       string          `json:"from" format:"date"`
	To         string          `json:"to" format:"date"`
	Today      string          `json:"today" format:"date"`
	Currencies []DailyCurrency `json:"currencies"`
	Accounts   []DailyAccount  `json:"accounts"`
}

func isSpendable(accountType string) bool {
	return accountType != accountTypeSavings && accountType != accountTypeInvestment
}

// DailyBalances computes end-of-day balances for [from, to] (defaulting to
// the current month) of the active accounts matching owner, plus a
// projection for the days after today.
func (s *Service) DailyBalances(ctx context.Context, from, to string, owner OwnerFilter) (DailySummary, error) {
	start, end, err := s.periodBounds(from, to)
	if err != nil {
		return DailySummary{}, err
	}
	if int(end.Sub(start).Hours()/24)+1 > maxDailySpanDays {
		return DailySummary{}, Invalid("to", "the range must not span more than 92 days")
	}
	today := s.Today()
	tomorrow := today.AddDate(0, 0, 1)

	accounts, err := s.ListOwnedAccounts(ctx, false, owner)
	if err != nil {
		return DailySummary{}, err
	}
	out := DailySummary{
		From: formatDate(start), To: formatDate(end), Today: formatDate(today),
		Currencies: []DailyCurrency{}, Accounts: make([]DailyAccount, 0, len(accounts)),
	}
	if len(accounts) == 0 {
		return out, nil
	}

	// Balances are walked from the day before the first day that matters:
	// the range start, or tomorrow when the range lies entirely ahead, so
	// the end-of-today balance is always known for the projection.
	first := start
	if tomorrow.Before(first) {
		first = tomorrow
	}
	opening, err := s.q.DailyOpeningBalances(ctx, store.DailyOpeningBalancesParams{
		AsOfDate: first.AddDate(0, 0, -1), OwnerID: owner.UserID, SharedOnly: owner.Shared,
	})
	if err != nil {
		return DailySummary{}, err
	}
	balance := make(map[uuid.UUID]int64, len(opening))
	for _, o := range opening {
		balance[o.AccountID] = o.Balance
	}
	type movement struct{ delta, income, expense int64 }
	moves := map[uuid.UUID]map[time.Time]movement{}
	deltas, err := s.q.DailyAccountDeltas(ctx, store.DailyAccountDeltasParams{
		FromDate: first, ToDate: end, OwnerID: owner.UserID, SharedOnly: owner.Shared,
	})
	if err != nil {
		return DailySummary{}, err
	}
	for _, d := range deltas {
		if moves[d.AccountID] == nil {
			moves[d.AccountID] = map[time.Time]movement{}
		}
		moves[d.AccountID][d.Day] = movement{d.Delta, d.Income, d.Expense}
	}

	byCurrency := map[string]*DailyCurrency{}
	spendable := map[uuid.UUID]string{} // account id -> currency, spendable accounts only
	accountIndex := make(map[uuid.UUID]int, len(accounts))
	for i, a := range accounts {
		cur, ok := byCurrency[a.Currency]
		if !ok {
			cur = &DailyCurrency{
				Currency: a.Currency, MinorUnits: a.MinorUnits, Days: []DailyBalance{}, Projection: []ProjectedBalance{},
				ProjectionBasis: ProjectionBasis{Recurring: []ProjectedRecurring{}},
			}
			byCurrency[a.Currency] = cur
		}
		if isSpendable(a.Type) {
			spendable[a.ID] = a.Currency
		}
		accountIndex[a.ID] = i
		out.Accounts = append(out.Accounts, DailyAccount{
			AccountID: a.ID, Name: a.Name, Type: a.Type, Currency: a.Currency, MinorUnits: a.MinorUnits, Days: []AccountBalance{},
		})
	}

	// Walk the days: record those inside [from, today], and keep the
	// end-of-today spendable balance and the future-dated deltas.
	spendableToday := map[string]int64{}
	if first.After(today) {
		// The walk starts after today: the opening balances are end of today.
		for _, a := range accounts {
			if isSpendable(a.Type) {
				spendableToday[a.Currency] += balance[a.ID]
			}
		}
	}
	futureDelta := map[string]map[time.Time]int64{} // currency -> day -> spendable delta after today
	for day := first; !day.After(end); day = day.AddDate(0, 0, 1) {
		date := formatDate(day)
		daily := map[string]*DailyBalance{}
		for cur := range byCurrency {
			daily[cur] = &DailyBalance{Date: date}
		}
		for _, a := range accounts {
			m := moves[a.ID][day]
			balance[a.ID] += m.delta
			d := daily[a.Currency]
			d.Income += m.income
			d.Expense += m.expense
			switch a.Type {
			case accountTypeSavings:
				d.SavingsBalance += balance[a.ID]
			case accountTypeInvestment:
				d.InvestmentBalance += balance[a.ID]
			default:
				d.Balance += balance[a.ID]
				if day.After(today) {
					if futureDelta[a.Currency] == nil {
						futureDelta[a.Currency] = map[time.Time]int64{}
					}
					futureDelta[a.Currency][day] += m.delta
				}
			}
			if !day.After(today) {
				out.Accounts[accountIndex[a.ID]].Days = append(out.Accounts[accountIndex[a.ID]].Days, AccountBalance{Date: date, Balance: balance[a.ID]})
			}
		}
		if day.After(today) {
			continue
		}
		for cur, d := range daily {
			spendableToday[cur] = d.Balance
			byCurrency[cur].Days = append(byCurrency[cur].Days, *d)
		}
	}

	if end.After(today) {
		if err := s.project(ctx, byCurrency, spendable, spendableToday, futureDelta, today, end, start, owner); err != nil {
			return DailySummary{}, err
		}
	}

	for _, c := range byCurrency {
		out.Currencies = append(out.Currencies, *c)
	}
	sort.Slice(out.Currencies, func(i, j int) bool { return out.Currencies[i].Currency < out.Currencies[j].Currency })
	return out, nil
}

// project fills Projection and ProjectionBasis of every currency. It starts
// from the end-of-today spendable balance and, for each day after today,
// subtracts the average daily spending, applies the unpaid recurring
// occurrences due that day (those already due are applied tomorrow) and the
// transactions already recorded for it.
func (s *Service) project(
	ctx context.Context,
	byCurrency map[string]*DailyCurrency,
	spendable map[uuid.UUID]string,
	spendableToday map[string]int64,
	futureDelta map[string]map[time.Time]int64,
	today, end, start time.Time,
	owner OwnerFilter,
) error {
	tomorrow := today.AddDate(0, 0, 1)

	baseline, err := s.q.DailyBaselineExpenses(ctx, store.DailyBaselineExpensesParams{
		FromDate: today.AddDate(0, 0, -(averageSpendingDays - 1)), ToDate: today, OwnerID: owner.UserID, SharedOnly: owner.Shared,
	})
	if err != nil {
		return err
	}
	spent := map[string]int64{}
	for _, b := range baseline {
		if cur, ok := spendable[b.AccountID]; ok {
			spent[cur] += b.Total
		}
	}

	active := StatusActive
	items, err := s.ListRecurringItems(ctx, RecurringFilter{Status: &active})
	if err != nil {
		return err
	}
	paid, err := s.enrichRecurring(ctx, items)
	if err != nil {
		return err
	}
	// applied[currency][day] -> signed recurring amount applied that day.
	applied := map[string]map[time.Time]int64{}
	for _, it := range items {
		cur, ok := spendable[it.AccountID]
		if !ok {
			continue
		}
		amount := it.Amount
		if it.Type == TypeExpense {
			amount = -amount
		}
		apply := func(due time.Time, on time.Time) {
			if paid[it.ID][due] {
				return
			}
			if applied[cur] == nil {
				applied[cur] = map[time.Time]int64{}
			}
			applied[cur][on] += amount
			c := byCurrency[cur]
			c.ProjectionBasis.Recurring = append(c.ProjectionBasis.Recurring, ProjectedRecurring{
				RecurringID: it.ID, Name: it.Name, DueOn: formatDate(due), AppliedOn: formatDate(on), Amount: amount,
			})
		}
		if it.CurrentPeriod != nil {
			if due, err := time.Parse(dateLayout, it.CurrentPeriod.DueOn); err == nil && !due.After(today) {
				apply(due, tomorrow)
			}
		}
		for _, due := range it.schedule().DueDatesBetween(tomorrow, end) {
			apply(due, due)
		}
	}

	for code, c := range byCurrency {
		avg := (spent[code] + averageSpendingDays/2) / averageSpendingDays
		c.ProjectionBasis.AverageDailyExpense = avg
		sort.SliceStable(c.ProjectionBasis.Recurring, func(i, j int) bool {
			return c.ProjectionBasis.Recurring[i].AppliedOn < c.ProjectionBasis.Recurring[j].AppliedOn
		})
		running := spendableToday[code]
		for day := tomorrow; !day.After(end); day = day.AddDate(0, 0, 1) {
			running += -avg + applied[code][day] + futureDelta[code][day]
			if !day.Before(start) {
				c.Projection = append(c.Projection, ProjectedBalance{Date: formatDate(day), Balance: running})
			}
		}
	}
	return nil
}
