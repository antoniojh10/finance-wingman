package finance

import (
	"context"
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/store"
)

type CategoryTotal struct {
	CategoryID       *uuid.UUID `json:"category_id" nullable:"true" doc:"Null for uncategorized transactions"`
	CategoryName     *string    `json:"category_name"`
	CategoryColor    *string    `json:"category_color"`
	Total            int64      `json:"total" doc:"Total in minor units"`
	TransactionCount int64      `json:"transaction_count"`
}

// CurrencySummary aggregates figures for a single currency. Amounts in
// different currencies are never mixed.
type CurrencySummary struct {
	Currency   string          `json:"currency" example:"MXN"`
	MinorUnits int             `json:"minor_units" example:"2"`
	Income     int64           `json:"income" doc:"Total income in the period, in minor units"`
	Expense    int64           `json:"expense" doc:"Total expenses in the period, in minor units"`
	Net        int64           `json:"net" doc:"Income minus expenses"`
	Balance    int64           `json:"balance" doc:"Current combined balance of active accounts in this currency"`
	Expenses   []CategoryTotal `json:"expenses" doc:"Expenses by category, largest first"`
	Incomes    []CategoryTotal `json:"incomes" doc:"Income by category, largest first"`
}

type Summary struct {
	From       string            `json:"from" format:"date"`
	To         string            `json:"to" format:"date"`
	Currencies []CurrencySummary `json:"currencies"`
}

// Summary computes totals for [from, to]. Empty bounds default to the
// current calendar month.
func (s *Service) Summary(ctx context.Context, from, to string) (Summary, error) {
	today := s.Today()
	start := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, -1)
	var err error
	if from != "" {
		if start, err = parseDate("from", from); err != nil {
			return Summary{}, err
		}
	}
	if to != "" {
		if end, err = parseDate("to", to); err != nil {
			return Summary{}, err
		}
	}
	if end.Before(start) {
		return Summary{}, Invalid("to", "must not be before from")
	}

	accounts, err := s.q.ListAccounts(ctx, false)
	if err != nil {
		return Summary{}, err
	}
	rows, err := s.q.SummaryByCategory(ctx, store.SummaryByCategoryParams{FromDate: start, ToDate: end})
	if err != nil {
		return Summary{}, err
	}
	currencies, err := s.q.ListCurrencies(ctx)
	if err != nil {
		return Summary{}, err
	}
	minorUnits := make(map[string]int, len(currencies))
	for _, c := range currencies {
		minorUnits[c.Code] = int(c.MinorUnits)
	}

	byCurrency := map[string]*CurrencySummary{}
	get := func(code string) *CurrencySummary {
		cs, ok := byCurrency[code]
		if !ok {
			cs = &CurrencySummary{Currency: code, MinorUnits: minorUnits[code], Expenses: []CategoryTotal{}, Incomes: []CategoryTotal{}}
			byCurrency[code] = cs
		}
		return cs
	}

	for _, a := range accounts {
		get(a.Currency).Balance += a.Balance
	}
	for _, r := range rows {
		cs := get(r.Currency)
		total := CategoryTotal{
			CategoryID:       r.CategoryID,
			CategoryName:     r.CategoryName,
			CategoryColor:    r.CategoryColor,
			Total:            r.Total,
			TransactionCount: r.TransactionCount,
		}
		if r.Type == TypeIncome {
			cs.Income += r.Total
			cs.Incomes = append(cs.Incomes, total)
		} else {
			cs.Expense += r.Total
			cs.Expenses = append(cs.Expenses, total)
		}
	}

	out := Summary{From: formatDate(start), To: formatDate(end), Currencies: make([]CurrencySummary, 0, len(byCurrency))}
	for _, cs := range byCurrency {
		cs.Net = cs.Income - cs.Expense
		out.Currencies = append(out.Currencies, *cs)
	}
	sort.Slice(out.Currencies, func(i, j int) bool { return out.Currencies[i].Currency < out.Currencies[j].Currency })
	return out, nil
}
