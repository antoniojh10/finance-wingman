package finance

import (
	"context"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/store"
)

var monthlyWindows = []int{3, 6, 12}

// MonthlyCategory is the spending of one category (or the uncategorized
// bucket) in each month of the window.
type MonthlyCategory struct {
	CategoryID *uuid.UUID `json:"category_id" nullable:"true" doc:"Null for uncategorized expenses"`
	Name       *string    `json:"name" nullable:"true" doc:"Null for uncategorized expenses"`
	Color      *string    `json:"color" nullable:"true"`
	Archived   bool       `json:"archived"`
	Totals     []int64    `json:"totals" doc:"Expenses per month, in minor units; one value per entry of months"`
	Budgets    []*int64   `json:"budgets" doc:"Budget in force per month, in minor units; null where the category has none, or everywhere when an owner filter is applied"`
}

// MonthlyCurrency groups the monthly spending of one currency.
type MonthlyCurrency struct {
	Currency     string            `json:"currency" example:"MXN"`
	MinorUnits   int               `json:"minor_units" example:"2"`
	Months       []string          `json:"months" doc:"Calendar months (YYYY-MM), oldest first, ending with the current one"`
	PartialMonth string            `json:"partial_month" pattern:"^\\d{4}-\\d{2}$" doc:"The month still in progress (the last one), whose figures are not final"`
	Categories   []MonthlyCategory `json:"categories" doc:"Categories with expenses in the window, largest period total first"`
	Totals       []int64           `json:"totals" doc:"Total expenses per month, in minor units"`
	Budgets      []*int64          `json:"budgets" doc:"Sum of the budgets in force per month for the currency, in minor units; null where none is set, or everywhere when an owner filter is applied"`
}

// MonthlySummary is the expense history by category of the last months.
type MonthlySummary struct {
	Months     []string          `json:"months" doc:"Calendar months (YYYY-MM), oldest first, ending with the current one"`
	Currencies []MonthlyCurrency `json:"currencies"`
}

// MonthlySpending returns, per currency, the expenses of the last `months`
// calendar months (the current one included, still in progress) by
// category. Transfers and income are not counted. Budgets are
// workspace-wide, so they are null whenever an owner filter is applied.
func (s *Service) MonthlySpending(ctx context.Context, months int, owner OwnerFilter) (MonthlySummary, error) {
	if !slices.Contains(monthlyWindows, months) {
		return MonthlySummary{}, Invalid("months", "must be one of 3, 6 or 12")
	}

	today := s.Today()
	current := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, time.UTC)
	starts := make([]time.Time, months)
	labels := make([]string, months)
	index := make(map[time.Time]int, months)
	for i := range starts {
		starts[i] = current.AddDate(0, i-(months-1), 0)
		labels[i] = starts[i].Format(monthLayout)
		index[starts[i]] = i
	}

	rows, err := s.q.SummaryMonthlyExpenses(ctx, store.SummaryMonthlyExpensesParams{
		FromDate: starts[0], ToDate: current.AddDate(0, 1, -1), OwnerID: owner.UserID, SharedOnly: owner.Shared,
	})
	if err != nil {
		return MonthlySummary{}, err
	}
	currencies, err := s.q.ListCurrencies(ctx)
	if err != nil {
		return MonthlySummary{}, err
	}
	minorUnits := make(map[string]int, len(currencies))
	for _, c := range currencies {
		minorUnits[c.Code] = int(c.MinorUnits)
	}

	byCurrency := map[string]*MonthlyCurrency{}
	byCategory := map[budgetKey]*MonthlyCategory{}
	for _, r := range rows {
		i, ok := index[r.Month]
		if !ok {
			continue
		}
		mc, ok := byCurrency[r.Currency]
		if !ok {
			mc = &MonthlyCurrency{
				Currency: r.Currency, MinorUnits: minorUnits[r.Currency], Months: labels,
				PartialMonth: labels[months-1], Categories: []MonthlyCategory{},
				Totals: make([]int64, months), Budgets: make([]*int64, months),
			}
			byCurrency[r.Currency] = mc
		}
		k := budgetKey{currency: r.Currency}
		if r.CategoryID != nil {
			k.category = *r.CategoryID
		}
		cat, ok := byCategory[k]
		if !ok {
			cat = &MonthlyCategory{
				CategoryID: r.CategoryID, Name: r.CategoryName, Color: r.CategoryColor,
				Archived: r.CategoryArchivedAt != nil,
				Totals:   make([]int64, months), Budgets: make([]*int64, months),
			}
			byCategory[k] = cat
		}
		cat.Totals[i] += r.Total
		mc.Totals[i] += r.Total
	}

	if owner.UserID == nil && !owner.Shared && len(byCurrency) > 0 {
		if err := s.fillMonthlyBudgets(ctx, starts, byCurrency, byCategory); err != nil {
			return MonthlySummary{}, err
		}
	}

	for k, cat := range byCategory {
		mc := byCurrency[k.currency]
		mc.Categories = append(mc.Categories, *cat)
	}
	out := MonthlySummary{Months: labels, Currencies: make([]MonthlyCurrency, 0, len(byCurrency))}
	for _, mc := range byCurrency {
		cats := mc.Categories
		periodTotal := func(c MonthlyCategory) (total int64) {
			for _, v := range c.Totals {
				total += v
			}
			return total
		}
		sort.Slice(cats, func(i, j int) bool {
			if a, b := periodTotal(cats[i]), periodTotal(cats[j]); a != b {
				return a > b
			}
			return strings.ToLower(deref(cats[i].Name)) < strings.ToLower(deref(cats[j].Name))
		})
		out.Currencies = append(out.Currencies, *mc)
	}
	sort.Slice(out.Currencies, func(i, j int) bool { return out.Currencies[i].Currency < out.Currencies[j].Currency })
	return out, nil
}

// fillMonthlyBudgets sets, for each month, the budget in force of every
// listed category and the sum of all the budgets in force of the currency
// (including categories without spending in the window).
func (s *Service) fillMonthlyBudgets(ctx context.Context, starts []time.Time, byCurrency map[string]*MonthlyCurrency, categories map[budgetKey]*MonthlyCategory) error {
	for i, month := range starts {
		budgets, err := s.q.ListEffectiveBudgets(ctx, month)
		if err != nil {
			return err
		}
		sums := map[string]int64{}
		for _, b := range budgets {
			// A null amount means the budget was cleared.
			mc, ok := byCurrency[b.Currency]
			if !ok || b.AmountMinor == nil {
				continue
			}
			sums[b.Currency] += *b.AmountMinor
			total := sums[b.Currency]
			mc.Budgets[i] = &total
			if cat, ok := categories[budgetKey{category: b.CategoryID, currency: b.Currency}]; ok {
				amount := *b.AmountMinor
				cat.Budgets[i] = &amount
			}
		}
	}
	return nil
}
