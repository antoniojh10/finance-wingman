package finance

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/store"
)

// suggestionMonths is how many complete months before the target month the
// suggestion looks at.
const suggestionMonths = 3

// SuggestionMonth is the spending of one past month used for a suggestion.
type SuggestionMonth struct {
	Month string `json:"month" pattern:"^\\d{4}-\\d{2}$" example:"2026-09"`
	Spent int64  `json:"spent" doc:"Expenses of the category in the month, in minor units (0 when there was none)"`
}

// BudgetSuggestion proposes a monthly amount for one expense category in one
// currency.
type BudgetSuggestion struct {
	CategoryID         uuid.UUID         `json:"category_id" format:"uuid"`
	CategoryName       string            `json:"category_name"`
	CategoryColor      *string           `json:"category_color"`
	Months             []SuggestionMonth `json:"months" doc:"Months the median was taken over, oldest first. Months before the first expense of the category in this currency are not included"`
	Median             int64             `json:"median" doc:"Median monthly spending over months, in minor units"`
	Recurring          int64             `json:"recurring" doc:"Monthly amount of active recurring expenses of the category with no payment in these months yet (e.g. a subscription created recently), in minor units. Added on top of the median"`
	Suggested          int64             `json:"suggested" doc:"median + recurring rounded up to a whole unit of the currency, in minor units"`
	CurrentAmount      *int64            `json:"current_amount" nullable:"true" doc:"Budget in force for the month, in minor units; null when the category has none"`
	CurrentAmountMonth *string           `json:"current_amount_month" nullable:"true" pattern:"^\\d{4}-\\d{2}$" doc:"Month the current amount was set in"`
}

// BudgetSuggestionCurrency groups the suggestions of one currency.
type BudgetSuggestionCurrency struct {
	Currency    string             `json:"currency" example:"MXN"`
	MinorUnits  int                `json:"minor_units" example:"2"`
	Suggestions []BudgetSuggestion `json:"suggestions" doc:"By category name"`
}

// BudgetSuggestions are the proposed budgets for a month.
type BudgetSuggestions struct {
	Month      string                     `json:"month" pattern:"^\\d{4}-\\d{2}$" example:"2026-10"`
	From       string                     `json:"from" pattern:"^\\d{4}-\\d{2}$" doc:"First of the 3 complete months before the month that were considered"`
	To         string                     `json:"to" pattern:"^\\d{4}-\\d{2}$" doc:"Last of the 3 complete months before the month that were considered"`
	Currencies []BudgetSuggestionCurrency `json:"currencies"`
}

// roundUpToUnit rounds amount (minor units) up to a whole unit of a
// currency with the given exponent: 12345 with 2 decimals becomes 12400.
func roundUpToUnit(amount int64, minorUnits int) int64 {
	unit := int64(1)
	for range minorUnits {
		unit *= 10
	}
	return (amount + unit - 1) / unit * unit
}

// SuggestBudgets proposes, for every expense category and currency with
// spending in the 3 complete months before month (empty means the current
// one), the median monthly spending rounded up to a whole currency unit.
// Months before the category's first expense in that currency are skipped;
// later months without spending count as zero. Active recurring expenses
// with no payment in those months yet are added on top, so a recent
// subscription is not missed. Transfers, income and uncategorized
// expenses are ignored. Suggestions cover the whole workspace.
func (s *Service) SuggestBudgets(ctx context.Context, month string) (BudgetSuggestions, error) {
	target, err := s.resolveMonth("month", month)
	if err != nil {
		return BudgetSuggestions{}, err
	}
	from := target.AddDate(0, -suggestionMonths, 0)
	to := target.AddDate(0, 0, -1)
	out := BudgetSuggestions{
		Month: target.Format(monthLayout), From: from.Format(monthLayout), To: to.Format(monthLayout),
		Currencies: []BudgetSuggestionCurrency{},
	}

	spend, err := s.q.ListMonthlyExpenseByCategory(ctx, store.ListMonthlyExpenseByCategoryParams{FromDate: from, ToDate: to})
	if err != nil {
		return out, err
	}
	if len(spend) == 0 {
		return out, nil
	}
	firsts, err := s.q.ListFirstExpenseByCategory(ctx, to)
	if err != nil {
		return out, err
	}
	firstOn := map[budgetKey]time.Time{}
	for _, f := range firsts {
		firstOn[budgetKey{*f.CategoryID, f.Currency}] = f.FirstOn
	}

	spentBy := map[budgetKey]map[time.Time]int64{}
	for _, r := range spend {
		key := budgetKey{*r.CategoryID, r.Currency}
		if spentBy[key] == nil {
			spentBy[key] = map[time.Time]int64{}
		}
		spentBy[key][r.Month] = r.Total
	}

	expense := TypeExpense
	categories, err := s.q.ListCategories(ctx, store.ListCategoriesParams{Kind: &expense})
	if err != nil {
		return out, err
	}
	type categoryInfo struct {
		name     string
		color    *string
		archived bool
	}
	info := map[uuid.UUID]categoryInfo{}
	for _, c := range categories {
		info[c.ID] = categoryInfo{c.Name, c.Color, c.ArchivedAt != nil}
	}
	currencies, err := s.q.ListCurrencies(ctx)
	if err != nil {
		return out, err
	}
	minorUnits := make(map[string]int, len(currencies))
	for _, c := range currencies {
		minorUnits[c.Code] = int(c.MinorUnits)
	}

	recurring, err := s.recurringNotReflected(ctx, from, to, target)
	if err != nil {
		return out, err
	}
	budgets, err := s.q.ListEffectiveBudgets(ctx, target)
	if err != nil {
		return out, err
	}
	current := map[budgetKey]store.ListEffectiveBudgetsRow{}
	for _, b := range budgets {
		if b.AmountMinor != nil {
			current[budgetKey{b.CategoryID, b.Currency}] = b
		}
	}

	byCurrency := map[string]*BudgetSuggestionCurrency{}
	for key, perMonth := range spentBy {
		cat, ok := info[key.category]
		if !ok || cat.archived {
			continue
		}
		var months []SuggestionMonth
		var values []int64
		for i := 0; i < suggestionMonths; i++ {
			m := from.AddDate(0, i, 0)
			// A month before the first expense says nothing about the category.
			if !m.AddDate(0, 1, 0).After(firstOn[key]) {
				continue
			}
			months = append(months, SuggestionMonth{Month: m.Format(monthLayout), Spent: perMonth[m]})
			values = append(values, perMonth[m])
		}
		if len(values) == 0 {
			continue
		}
		median := medianAmount(values)
		extra := recurring[key]
		sg := BudgetSuggestion{
			CategoryID: key.category, CategoryName: cat.name, CategoryColor: cat.color,
			Months: months, Median: median, Recurring: extra,
			Suggested: roundUpToUnit(median+extra, minorUnits[key.currency]),
		}
		if b, ok := current[key]; ok {
			src := b.SourceMonth.Format(monthLayout)
			sg.CurrentAmount, sg.CurrentAmountMonth = b.AmountMinor, &src
		}
		bc, ok := byCurrency[key.currency]
		if !ok {
			bc = &BudgetSuggestionCurrency{Currency: key.currency, MinorUnits: minorUnits[key.currency], Suggestions: []BudgetSuggestion{}}
			byCurrency[key.currency] = bc
		}
		bc.Suggestions = append(bc.Suggestions, sg)
	}
	for _, bc := range byCurrency {
		sort.Slice(bc.Suggestions, func(i, j int) bool {
			return strings.ToLower(bc.Suggestions[i].CategoryName) < strings.ToLower(bc.Suggestions[j].CategoryName)
		})
		out.Currencies = append(out.Currencies, *bc)
	}
	sort.Slice(out.Currencies, func(i, j int) bool { return out.Currencies[i].Currency < out.Currencies[j].Currency })
	return out, nil
}

// recurringNotReflected sums, per category and currency, the monthly amount
// of active recurring expenses that have no linked payment between from and
// to (their cost is not in the past spending) and that start by the end of
// the target month.
func (s *Service) recurringNotReflected(ctx context.Context, from, to, target time.Time) (map[budgetKey]int64, error) {
	out := map[budgetKey]int64{}
	items, err := s.q.ListBudgetRecurringExpenses(ctx, store.ListBudgetRecurringExpensesParams{})
	if err != nil || len(items) == 0 {
		return out, err
	}
	paidIDs, err := s.q.ListRecurringIDsPaidBetween(ctx, store.ListRecurringIDsPaidBetweenParams{FromDate: from, ToDate: to})
	if err != nil {
		return nil, err
	}
	paid := map[uuid.UUID]bool{}
	for _, id := range paidIDs {
		paid[id] = true
	}
	targetEnd := target.AddDate(0, 1, -1)
	for _, it := range items {
		if it.CategoryID == nil || paid[it.ID] || it.StartOn.After(targetEnd) {
			continue
		}
		out[budgetKey{*it.CategoryID, it.Currency}] += MonthlyAmount(it.Amount, it.IntervalUnit, int(it.IntervalCount))
	}
	return out, nil
}
