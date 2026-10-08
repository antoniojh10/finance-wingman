package finance

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/store"
)

// Budget states. A line is "near" once spending plus committed recurring
// reaches 80% of its amount, and "over" once it exceeds the amount.
const (
	BudgetOK   = "ok"
	BudgetNear = "near"
	BudgetOver = "over"
	// BudgetNone marks a line without a budget (spending only).
	BudgetNone = "none"
)

const (
	monthLayout = "2006-01"
	// budgetNearPercent is the share of the amount from which a budget is near.
	budgetNearPercent = 80
	maxBudgetAmount   = maxRecurringAmount
)

// BudgetLine is one category (or the uncategorized bucket) in one currency.
type BudgetLine struct {
	CategoryID    *uuid.UUID `json:"category_id" nullable:"true" doc:"Null for the uncategorized bucket"`
	CategoryName  *string    `json:"category_name"`
	CategoryColor *string    `json:"category_color"`
	Amount        *int64     `json:"amount" nullable:"true" doc:"Budget in force for the month, in minor units. Null when the category has no budget"`
	AmountMonth   *string    `json:"amount_month" nullable:"true" pattern:"^\\d{4}-\\d{2}$" doc:"Month (YYYY-MM) the amount was set in. Differs from the requested month when the amount is inherited from an earlier one"`
	Spent         int64      `json:"spent" doc:"Expenses in the month, in minor units (transfers excluded)"`
	Committed     int64      `json:"committed" doc:"Active recurring expenses of the category due this month and not paid yet, in minor units. Not counted as spent"`
	Remaining     *int64     `json:"remaining" nullable:"true" doc:"amount - spent - committed; negative when over budget. Null when there is no budget"`
	State         string     `json:"state" enum:"ok,near,over,none" doc:"none: no budget. near: spent + committed is at least 80% of the amount. over: it exceeds the amount"`
}

// BudgetCurrency groups the lines of one currency. Currencies are never
// converted.
type BudgetCurrency struct {
	Currency   string `json:"currency" example:"MXN"`
	MinorUnits int    `json:"minor_units" example:"2"`
	// Totals cover the categories that have a budget; unbudgeted and
	// uncategorized spending is only in their own lines.
	Budgeted      int64        `json:"budgeted" doc:"Sum of the budget amounts, in minor units"`
	Spent         int64        `json:"spent" doc:"Spent in the categories that have a budget"`
	Committed     int64        `json:"committed" doc:"Committed in the categories that have a budget"`
	Remaining     int64        `json:"remaining" doc:"budgeted - spent - committed"`
	Categories    []BudgetLine `json:"categories" doc:"Categories with a budget or with spending/committed expenses in the month, by name"`
	Uncategorized *BudgetLine  `json:"uncategorized" doc:"Expenses without a category; null when there are none"`
	_             struct{}     `json:"-"`
}

// BudgetStatus is the budget situation of one month.
type BudgetStatus struct {
	Month      string           `json:"month" pattern:"^\\d{4}-\\d{2}$" example:"2026-10"`
	Currencies []BudgetCurrency `json:"currencies"`
}

// BudgetItemInput sets or clears the budget of one category and currency
// from a month on.
type BudgetItemInput struct {
	CategoryID uuid.UUID `json:"category_id" format:"uuid" doc:"Expense category"`
	Currency   string    `json:"currency" example:"MXN" doc:"ISO 4217 code. A category has an independent budget per currency"`
	Amount     *int64    `json:"amount,omitempty" minimum:"0" maximum:"1000000000000000" doc:"Budget in minor units (0 means spend nothing). Later months inherit it until one sets another amount"`
	Clear      bool      `json:"clear,omitempty" doc:"Remove the budget from this month on (earlier months keep theirs). Use instead of amount"`
}

// parseMonth reads YYYY-MM and returns the first day of that month.
func parseMonth(field, value string) (time.Time, error) {
	m, err := time.Parse(monthLayout, value)
	if err != nil {
		return time.Time{}, Invalid(field, "must be a month in YYYY-MM format")
	}
	return m, nil
}

func (s *Service) resolveMonth(field, value string) (time.Time, error) {
	if value == "" {
		t := s.Today()
		return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC), nil
	}
	return parseMonth(field, value)
}

func budgetState(amount, used int64) string {
	switch {
	case used > amount:
		return BudgetOver
	case used > 0 && used*100 >= amount*budgetNearPercent:
		return BudgetNear
	default:
		return BudgetOK
	}
}

type budgetKey struct {
	category uuid.UUID // uuid.Nil for uncategorized
	currency string
}

// SetBudgets sets or clears budgets from month on, atomically. Each item
// only writes the given month: later months inherit it until they set their
// own amount.
func (s *Service) SetBudgets(ctx context.Context, month string, items []BudgetItemInput) (BudgetStatus, error) {
	first, err := parseMonth("month", month)
	if err != nil {
		return BudgetStatus{}, err
	}
	if err := checkBatchSize(len(items)); err != nil {
		return BudgetStatus{}, err
	}
	currencies, err := s.q.ListCurrencies(ctx)
	if err != nil {
		return BudgetStatus{}, err
	}
	known := map[string]bool{}
	for _, c := range currencies {
		known[c.Code] = true
	}

	type write struct {
		category uuid.UUID
		currency string
		amount   *int64
	}
	writes := make([]write, len(items))
	seen := map[budgetKey]int{}
	for i, in := range items {
		code := strings.ToUpper(strings.TrimSpace(in.Currency))
		if !known[code] {
			return BudgetStatus{}, itemError(i, Invalid("currency", "unknown currency "+fmt.Sprintf("%q", in.Currency)+"; use an ISO 4217 code such as MXN"))
		}
		switch {
		case in.Clear && in.Amount != nil:
			return BudgetStatus{}, itemError(i, Invalid("clear", "cannot be combined with amount"))
		case !in.Clear && in.Amount == nil:
			return BudgetStatus{}, itemError(i, Invalid("amount", "is required unless clear is true"))
		case in.Amount != nil && (*in.Amount < 0 || *in.Amount > maxBudgetAmount):
			return BudgetStatus{}, itemError(i, Invalid("amount", "must be zero or a positive amount in minor units"))
		}
		key := budgetKey{in.CategoryID, code}
		if first, dup := seen[key]; dup {
			return BudgetStatus{}, itemError(i, Invalid("category_id", fmt.Sprintf("duplicates items[%d]: set each category and currency once per request", first)))
		}
		seen[key] = i
		writes[i] = write{in.CategoryID, code, in.Amount}
	}

	err = s.withTx(ctx, func(tx *Service) error {
		for i, w := range writes {
			category, err := tx.q.GetCategory(ctx, w.category)
			if isNoRows(err) {
				return itemError(i, Invalid("category_id", "category not found; list the expense categories to pick a valid one"))
			}
			if err != nil {
				return err
			}
			if category.Kind != TypeExpense {
				return itemError(i, Invalid("category_id", fmt.Sprintf("%q is an income category; budgets only apply to expense categories", category.Name)))
			}
			params := store.UpsertBudgetParams{CategoryID: w.category, Currency: w.currency, Month: first, AmountMinor: w.amount}
			if actor, ok := ActorFrom(ctx); ok {
				params.CreatedBy = &actor
			}
			if err := tx.q.UpsertBudget(ctx, params); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return BudgetStatus{}, err
	}
	return s.BudgetStatus(ctx, month, OwnerFilter{})
}

// BudgetStatus computes the budgets of a month (empty means the current
// one) per currency and category. The owner filter only narrows the
// spent and committed figures to the accounts of that owner; budgets are
// workspace-wide.
func (s *Service) BudgetStatus(ctx context.Context, month string, owner OwnerFilter) (BudgetStatus, error) {
	first, err := s.resolveMonth("month", month)
	if err != nil {
		return BudgetStatus{}, err
	}
	last := first.AddDate(0, 1, -1)

	budgets, err := s.q.ListEffectiveBudgets(ctx, first)
	if err != nil {
		return BudgetStatus{}, err
	}
	spentRows, err := s.q.SummaryByCategory(ctx, store.SummaryByCategoryParams{
		FromDate: first, ToDate: last, OwnerID: owner.UserID, SharedOnly: owner.Shared,
	})
	if err != nil {
		return BudgetStatus{}, err
	}
	committed, err := s.committedByCategory(ctx, first, last, owner)
	if err != nil {
		return BudgetStatus{}, err
	}
	currencies, err := s.q.ListCurrencies(ctx)
	if err != nil {
		return BudgetStatus{}, err
	}
	minorUnits := make(map[string]int, len(currencies))
	for _, c := range currencies {
		minorUnits[c.Code] = int(c.MinorUnits)
	}

	lines := map[budgetKey]*BudgetLine{}
	archived := map[budgetKey]bool{}
	line := func(key budgetKey, id *uuid.UUID, name, color *string) *BudgetLine {
		l, ok := lines[key]
		if !ok {
			l = &BudgetLine{CategoryID: id, CategoryName: name, CategoryColor: color}
			lines[key] = l
		}
		return l
	}
	for _, b := range budgets {
		if b.AmountMinor == nil {
			continue
		}
		id, name := b.CategoryID, b.CategoryName
		l := line(budgetKey{b.CategoryID, b.Currency}, &id, &name, b.CategoryColor)
		l.Amount = b.AmountMinor
		src := b.SourceMonth.Format(monthLayout)
		l.AmountMonth = &src
		archived[budgetKey{b.CategoryID, b.Currency}] = b.CategoryArchivedAt != nil
	}
	for _, r := range spentRows {
		if r.Type != TypeExpense {
			continue
		}
		key := budgetKey{currency: r.Currency}
		if r.CategoryID != nil {
			key.category = *r.CategoryID
		}
		line(key, r.CategoryID, r.CategoryName, r.CategoryColor).Spent += r.Total
	}
	for key, c := range committed {
		line(key, c.categoryID, c.name, c.color).Committed += c.amount
	}

	byCurrency := map[string]*BudgetCurrency{}
	for key, l := range lines {
		// An archived category is only listed while it still has activity.
		if archived[key] && l.Spent == 0 && l.Committed == 0 {
			continue
		}
		bc, ok := byCurrency[key.currency]
		if !ok {
			bc = &BudgetCurrency{Currency: key.currency, MinorUnits: minorUnits[key.currency], Categories: []BudgetLine{}}
			byCurrency[key.currency] = bc
		}
		l.State = BudgetNone
		if l.Amount != nil {
			used := l.Spent + l.Committed
			remaining := *l.Amount - used
			l.Remaining = &remaining
			l.State = budgetState(*l.Amount, used)
			bc.Budgeted += *l.Amount
			bc.Spent += l.Spent
			bc.Committed += l.Committed
		}
		if key.category == uuid.Nil {
			bc.Uncategorized = l
		} else {
			bc.Categories = append(bc.Categories, *l)
		}
	}

	out := BudgetStatus{Month: first.Format(monthLayout), Currencies: make([]BudgetCurrency, 0, len(byCurrency))}
	for _, bc := range byCurrency {
		bc.Remaining = bc.Budgeted - bc.Spent - bc.Committed
		sort.Slice(bc.Categories, func(i, j int) bool {
			return strings.ToLower(*bc.Categories[i].CategoryName) < strings.ToLower(*bc.Categories[j].CategoryName)
		})
		out.Currencies = append(out.Currencies, *bc)
	}
	sort.Slice(out.Currencies, func(i, j int) bool { return out.Currencies[i].Currency < out.Currencies[j].Currency })
	return out, nil
}

type committedAmount struct {
	categoryID *uuid.UUID
	name       *string
	color      *string
	amount     int64
}

// committedByCategory sums, per category and currency, the amount of the
// due dates of active recurring expenses that fall in [first, last] and
// have no linked payment yet. A due date counts as paid under the same rule
// as the recurring periods: a transaction linked to that item and due date.
// Months that already ended have no committed amount: what was not paid
// then is overdue, not upcoming.
func (s *Service) committedByCategory(ctx context.Context, first, last time.Time, owner OwnerFilter) (map[budgetKey]*committedAmount, error) {
	out := map[budgetKey]*committedAmount{}
	if last.Before(s.Today()) {
		return out, nil
	}
	items, err := s.q.ListBudgetRecurringExpenses(ctx, store.ListBudgetRecurringExpensesParams{OwnerID: owner.UserID, SharedOnly: owner.Shared})
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return out, nil
	}
	ids := make([]uuid.UUID, len(items))
	for i, it := range items {
		ids[i] = it.ID
	}
	periods, err := s.q.ListRecurringPaidPeriods(ctx, ids)
	if err != nil {
		return nil, err
	}
	paid := paidSet{}
	for _, p := range periods {
		if p.RecurringID == nil || p.RecurringDueOn == nil {
			continue
		}
		if paid[*p.RecurringID] == nil {
			paid[*p.RecurringID] = map[time.Time]bool{}
		}
		paid[*p.RecurringID][*p.RecurringDueOn] = true
	}
	for _, it := range items {
		sched := Schedule{Unit: it.IntervalUnit, Count: int(it.IntervalCount), StartOn: it.StartOn}
		if it.TotalPayments != nil {
			n := int(*it.TotalPayments)
			sched.TotalPayments = &n
		}
		var pending int64
		for _, due := range sched.DueDatesBetween(first, last) {
			if !paid[it.ID][due] {
				pending += it.Amount
			}
		}
		if pending == 0 {
			continue
		}
		key := budgetKey{currency: it.Currency}
		if it.CategoryID != nil {
			key.category = *it.CategoryID
		}
		c, ok := out[key]
		if !ok {
			c = &committedAmount{categoryID: it.CategoryID, name: it.CategoryName, color: it.CategoryColor}
			out[key] = c
		}
		c.amount += pending
	}
	return out, nil
}

// BudgetImpactResult tells what recording an expense would do to the budget
// of its category.
type BudgetImpactResult struct {
	Month       string `json:"month" pattern:"^\\d{4}-\\d{2}$"`
	Currency    string `json:"currency"`
	HasBudget   bool   `json:"has_budget" doc:"False when the category has no budget in force that month; the other figures are then zero"`
	Amount      int64  `json:"amount" doc:"Budget in force, in minor units"`
	Spent       int64  `json:"spent" doc:"Already spent in the month, before the expense"`
	Committed   int64  `json:"committed" doc:"Committed recurring expenses not paid yet"`
	Remaining   int64  `json:"remaining" doc:"Remaining after the expense; negative when over"`
	StateBefore string `json:"state_before" enum:"ok,near,over,none"`
	StateAfter  string `json:"state_after" enum:"ok,near,over,none"`
	Warning     bool   `json:"warning" doc:"True when the category is near or over its budget after the expense"`
}

// BudgetImpact evaluates an expense that is not recorded yet: it reports
// the state of the category budget (workspace-wide, every owner) in the
// month of date before and after adding amount (minor units) to it. An
// expense that is already recorded must not be passed here, or it would be
// counted twice.
func (s *Service) BudgetImpact(ctx context.Context, categoryID uuid.UUID, currency string, date time.Time, amount int64) (BudgetImpactResult, error) {
	if amount < 0 {
		return BudgetImpactResult{}, Invalid("amount", "must not be negative")
	}
	currency = strings.ToUpper(strings.TrimSpace(currency))
	month := time.Date(date.Year(), date.Month(), 1, 0, 0, 0, 0, time.UTC).Format(monthLayout)
	res := BudgetImpactResult{Month: month, Currency: currency, StateBefore: BudgetNone, StateAfter: BudgetNone}

	status, err := s.BudgetStatus(ctx, month, OwnerFilter{})
	if err != nil {
		return res, err
	}
	for _, cur := range status.Currencies {
		if cur.Currency != currency {
			continue
		}
		for _, l := range cur.Categories {
			if l.CategoryID == nil || *l.CategoryID != categoryID || l.Amount == nil {
				continue
			}
			used := l.Spent + l.Committed
			res.HasBudget = true
			res.Amount = *l.Amount
			res.Spent = l.Spent
			res.Committed = l.Committed
			res.Remaining = *l.Amount - used - amount
			res.StateBefore = l.State
			res.StateAfter = budgetState(*l.Amount, used+amount)
			res.Warning = res.StateAfter == BudgetNear || res.StateAfter == BudgetOver
		}
	}
	return res, nil
}
