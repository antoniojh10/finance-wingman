package finance

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/store"
)

// Recurring item statuses. Items are never deleted: they are retired with
// StatusCancelled and kept with their history.
const (
	StatusActive    = "active"
	StatusPaused    = "paused"
	StatusCancelled = "cancelled"
)

var (
	recurringStatuses = []string{StatusActive, StatusPaused, StatusCancelled}
	recurringUnits    = []string{UnitWeek, UnitMonth, UnitYear}
)

const (
	// maxRecurringAmount keeps MonthlyAmount's intermediate product far from
	// int64 overflow (10^15 minor units).
	maxRecurringAmount int64 = 1_000_000_000_000_000
	maxIntervalCount         = 1000
	maxTotalPayments         = 10000
	maxNotesLength           = 1000
)

type RecurringItem struct {
	ID            uuid.UUID         `json:"id"`
	Name          string            `json:"name" example:"Netflix"`
	Type          string            `json:"type" enum:"expense,income"`
	AccountID     uuid.UUID         `json:"account_id"`
	AccountName   string            `json:"account_name"`
	Currency      string            `json:"currency" example:"MXN"`
	MinorUnits    int               `json:"minor_units" example:"2"`
	CategoryID    *uuid.UUID        `json:"category_id" nullable:"true"`
	CategoryName  *string           `json:"category_name"`
	Amount        int64             `json:"amount" doc:"Estimated amount of each occurrence, in minor units"`
	MonthlyAmount int64             `json:"monthly_amount" doc:"Amount normalized to an average month, in minor units (rounded half up)"`
	Notes         string            `json:"notes"`
	IntervalUnit  string            `json:"interval_unit" enum:"week,month,year"`
	IntervalCount int               `json:"interval_count" doc:"Repeats every this many units"`
	StartOn       string            `json:"start_on" format:"date" doc:"First due date"`
	TotalPayments *int              `json:"total_payments" nullable:"true" doc:"Number of payments for installment plans; null when open-ended"`
	LastDueOn     *string           `json:"last_due_on" format:"date" nullable:"true" doc:"Final due date of an installment plan; null when open-ended"`
	NextDueOn     *string           `json:"next_due_on" format:"date" nullable:"true" doc:"Next due date on or after today. Null unless the item is active and still has due dates"`
	Status        string            `json:"status" enum:"active,paused,cancelled"`
	CurrentPeriod *RecurringPeriod  `json:"current_period" doc:"Period the item is in now (latest due date on or before today). Null unless the item is active"`
	LastPayment   *RecurringPayment `json:"last_payment" doc:"Most recent linked transaction"`
	CreatedAt     time.Time         `json:"created_at"`
	UpdatedAt     time.Time         `json:"updated_at"`
}

type CreateRecurringItemInput struct {
	Name          string     `json:"name" minLength:"1" maxLength:"100"`
	Type          string     `json:"type" enum:"expense,income"`
	AccountID     uuid.UUID  `json:"account_id" format:"uuid" doc:"Account that is charged or credited; its currency applies to the item"`
	CategoryID    *uuid.UUID `json:"category_id,omitempty" format:"uuid" doc:"Category whose kind matches the type"`
	Amount        int64      `json:"amount" minimum:"1" maximum:"1000000000000000" doc:"Estimated amount of each occurrence, in minor units"`
	Notes         string     `json:"notes,omitempty" maxLength:"1000"`
	IntervalUnit  string     `json:"interval_unit" enum:"week,month,year"`
	IntervalCount int        `json:"interval_count,omitempty" minimum:"1" maximum:"1000" default:"1" doc:"Repeats every this many units"`
	StartOn       string     `json:"start_on,omitempty" format:"date" doc:"First due date. Defaults to today"`
	TotalPayments *int       `json:"total_payments,omitempty" minimum:"1" maximum:"10000" doc:"Number of payments for an installment plan. Omit for open-ended items"`
}

// UpdateRecurringItemInput partially updates an item. The type and account
// cannot be changed; cancel the item and create a new one instead.
type UpdateRecurringItemInput struct {
	Name          *string    `json:"name,omitempty" minLength:"1" maxLength:"100"`
	CategoryID    *uuid.UUID `json:"category_id,omitempty" format:"uuid"`
	ClearCategory bool       `json:"clear_category,omitempty" doc:"Remove the category"`
	Amount        *int64     `json:"amount,omitempty" minimum:"1" maximum:"1000000000000000"`
	Notes         *string    `json:"notes,omitempty" maxLength:"1000"`
	IntervalUnit  *string    `json:"interval_unit,omitempty" enum:"week,month,year"`
	IntervalCount *int       `json:"interval_count,omitempty" minimum:"1" maximum:"1000"`
	StartOn       *string    `json:"start_on,omitempty" format:"date"`
	TotalPayments *int       `json:"total_payments,omitempty" minimum:"1" maximum:"10000"`
	// ClearTotalPayments turns an installment plan into an open-ended item.
	ClearTotalPayments bool    `json:"clear_total_payments,omitempty"`
	Status             *string `json:"status,omitempty" enum:"active,paused,cancelled" doc:"Set to cancelled to retire the item"`
}

type RecurringFilter struct {
	Status *string
	Type   *string
}

// RecurringCurrencySummary is the committed monthly cost in one currency.
type RecurringCurrencySummary struct {
	Currency     string `json:"currency" example:"MXN"`
	MinorUnits   int    `json:"minor_units" example:"2"`
	Expense      int64  `json:"expense" doc:"Committed monthly expenses, in minor units"`
	Income       int64  `json:"income" doc:"Expected monthly recurring income, in minor units"`
	Net          int64  `json:"net" doc:"Income minus expenses"`
	ExpenseCount int    `json:"expense_count" doc:"Active recurring expenses included"`
	IncomeCount  int    `json:"income_count" doc:"Active recurring incomes included"`
}

// RecurringSummary is the committed monthly cost of all active items,
// grouped per currency (no conversion).
type RecurringSummary struct {
	Currencies []RecurringCurrencySummary `json:"currencies"`
}

func (s *Service) recurringFromRow(r store.GetRecurringItemRow) RecurringItem {
	sched := Schedule{Unit: r.IntervalUnit, Count: int(r.IntervalCount), StartOn: r.StartOn}
	var total *int
	if r.TotalPayments != nil {
		n := int(*r.TotalPayments)
		total = &n
		sched.TotalPayments = total
	}
	item := RecurringItem{
		ID:            r.ID,
		Name:          r.Name,
		Type:          r.Type,
		AccountID:     r.AccountID,
		AccountName:   r.AccountName,
		Currency:      r.Currency,
		MinorUnits:    int(r.MinorUnits),
		CategoryID:    r.CategoryID,
		CategoryName:  r.CategoryName,
		Amount:        r.Amount,
		MonthlyAmount: MonthlyAmount(r.Amount, r.IntervalUnit, int(r.IntervalCount)),
		Notes:         r.Notes,
		IntervalUnit:  r.IntervalUnit,
		IntervalCount: int(r.IntervalCount),
		StartOn:       formatDate(r.StartOn),
		TotalPayments: total,
		Status:        r.Status,
		CreatedAt:     r.CreatedAt,
		UpdatedAt:     r.UpdatedAt,
	}
	if last, ok := sched.LastDueOn(); ok {
		d := formatDate(last)
		item.LastDueOn = &d
	}
	if r.Status == StatusActive {
		if next, ok := sched.NextDueOn(s.Today()); ok {
			d := formatDate(next)
			item.NextDueOn = &d
		}
	}
	return item
}

func (s *Service) ListRecurringItems(ctx context.Context, f RecurringFilter) ([]RecurringItem, error) {
	if f.Status != nil && !slices.Contains(recurringStatuses, *f.Status) {
		return nil, Invalid("status", "must be one of "+strings.Join(recurringStatuses, ", "))
	}
	if f.Type != nil {
		if err := validateKind("type", *f.Type); err != nil {
			return nil, err
		}
	}
	rows, err := s.q.ListRecurringItems(ctx, store.ListRecurringItemsParams{Status: f.Status, Type: f.Type})
	if err != nil {
		return nil, err
	}
	out := make([]RecurringItem, len(rows))
	for i, r := range rows {
		out[i] = s.recurringFromRow(store.GetRecurringItemRow(r))
	}
	if _, err := s.enrichRecurring(ctx, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Service) getRecurringRow(ctx context.Context, id uuid.UUID) (store.GetRecurringItemRow, error) {
	row, err := s.q.GetRecurringItem(ctx, id)
	if isNoRows(err) {
		return row, NotFound("recurring item")
	}
	return row, err
}

func (s *Service) GetRecurringItem(ctx context.Context, id uuid.UUID) (RecurringItem, error) {
	row, err := s.getRecurringRow(ctx, id)
	if err != nil {
		return RecurringItem{}, err
	}
	items := []RecurringItem{s.recurringFromRow(row)}
	if _, err := s.enrichRecurring(ctx, items); err != nil {
		return RecurringItem{}, err
	}
	return items[0], nil
}

func validateRecurringAmount(amount int64) error {
	if amount < 1 || amount > maxRecurringAmount {
		return Invalid("amount", "must be a positive amount in minor units")
	}
	return nil
}

func validateRecurringSchedule(unit string, count int, total *int) error {
	if !slices.Contains(recurringUnits, unit) {
		return Invalid("interval_unit", "must be one of "+strings.Join(recurringUnits, ", "))
	}
	if count < 1 || count > maxIntervalCount {
		return Invalid("interval_count", "must be between 1 and 1000")
	}
	if total != nil && (*total < 1 || *total > maxTotalPayments) {
		return Invalid("total_payments", "must be between 1 and 10000")
	}
	return nil
}

func validateNotes(notes string) error {
	if len([]rune(notes)) > maxNotesLength {
		return Invalid("notes", "must be at most 1000 characters")
	}
	return nil
}

// recurringCategory checks that a category exists, matches the item type
// and is not archived (unless it is the one the item already has).
func (s *Service) recurringCategory(ctx context.Context, id uuid.UUID, itemType string, current *uuid.UUID) error {
	category, err := s.q.GetCategory(ctx, id)
	if isNoRows(err) {
		return Invalid("category_id", "category not found")
	}
	if err != nil {
		return err
	}
	if category.Kind != itemType {
		return Invalid("category_id", "category kind "+category.Kind+" does not match recurring item type "+itemType)
	}
	if category.ArchivedAt != nil && (current == nil || *current != category.ID) {
		return Invalid("category_id", "category is archived")
	}
	return nil
}

// recurringNameConflict builds the conflict error for a name that clashes
// with another non-cancelled item, naming that item so callers can act on it.
func (s *Service) recurringNameConflict(ctx context.Context, name string, self uuid.UUID) error {
	other, err := s.q.FindOpenRecurringItemByName(ctx, store.FindOpenRecurringItemByNameParams{Lower: name, ID: self})
	if err != nil {
		return Conflict("a recurring item with this name already exists; names are unique among non-cancelled items")
	}
	return Conflict(fmt.Sprintf("a recurring item named %q already exists (status %s, id %s); names are unique among non-cancelled items, so rename or cancel it first", other.Name, other.Status, other.ID))
}

func (s *Service) CreateRecurringItem(ctx context.Context, in CreateRecurringItemInput) (RecurringItem, error) {
	name, err := normalizeName("name", in.Name)
	if err != nil {
		return RecurringItem{}, err
	}
	if err := validateKind("type", in.Type); err != nil {
		return RecurringItem{}, err
	}
	if err := validateRecurringAmount(in.Amount); err != nil {
		return RecurringItem{}, err
	}
	if err := validateNotes(in.Notes); err != nil {
		return RecurringItem{}, err
	}
	count := in.IntervalCount
	if count == 0 {
		count = 1
	}
	if err := validateRecurringSchedule(in.IntervalUnit, count, in.TotalPayments); err != nil {
		return RecurringItem{}, err
	}
	startOn := s.Today()
	if in.StartOn != "" {
		if startOn, err = parseDate("start_on", in.StartOn); err != nil {
			return RecurringItem{}, err
		}
	}
	if _, err := s.usableAccount(ctx, "account_id", in.AccountID, false); err != nil {
		return RecurringItem{}, err
	}
	if in.CategoryID != nil {
		if err := s.recurringCategory(ctx, *in.CategoryID, in.Type, nil); err != nil {
			return RecurringItem{}, err
		}
	}

	params := store.CreateRecurringItemParams{
		Name:          name,
		Type:          in.Type,
		AccountID:     in.AccountID,
		CategoryID:    in.CategoryID,
		Amount:        in.Amount,
		Notes:         strings.TrimSpace(in.Notes),
		IntervalUnit:  in.IntervalUnit,
		IntervalCount: int32(count),
		StartOn:       startOn,
	}
	if in.TotalPayments != nil {
		n := int32(*in.TotalPayments)
		params.TotalPayments = &n
	}
	if actor, ok := ActorFrom(ctx); ok {
		params.CreatedBy = &actor
	}
	id, err := s.q.CreateRecurringItem(ctx, params)
	if pgErrorCode(err) == pgUniqueViolation {
		return RecurringItem{}, s.recurringNameConflict(ctx, name, uuid.Nil)
	}
	if err != nil {
		return RecurringItem{}, err
	}
	return s.GetRecurringItem(ctx, id)
}

func (s *Service) UpdateRecurringItem(ctx context.Context, id uuid.UUID, in UpdateRecurringItemInput) (RecurringItem, error) {
	cur, err := s.getRecurringRow(ctx, id)
	if err != nil {
		return RecurringItem{}, err
	}

	p := store.UpdateRecurringItemParams{
		ID:            id,
		Name:          cur.Name,
		CategoryID:    cur.CategoryID,
		Amount:        cur.Amount,
		Notes:         cur.Notes,
		IntervalUnit:  cur.IntervalUnit,
		IntervalCount: cur.IntervalCount,
		StartOn:       cur.StartOn,
		TotalPayments: cur.TotalPayments,
		Status:        cur.Status,
	}

	if in.Name != nil {
		if p.Name, err = normalizeName("name", *in.Name); err != nil {
			return RecurringItem{}, err
		}
	}
	if in.Amount != nil {
		if err := validateRecurringAmount(*in.Amount); err != nil {
			return RecurringItem{}, err
		}
		p.Amount = *in.Amount
	}
	if in.Notes != nil {
		if err := validateNotes(*in.Notes); err != nil {
			return RecurringItem{}, err
		}
		p.Notes = strings.TrimSpace(*in.Notes)
	}
	if in.CategoryID != nil && in.ClearCategory {
		return RecurringItem{}, Invalid("clear_category", "cannot be combined with category_id")
	}
	if in.CategoryID != nil {
		if err := s.recurringCategory(ctx, *in.CategoryID, cur.Type, cur.CategoryID); err != nil {
			return RecurringItem{}, err
		}
		p.CategoryID = in.CategoryID
	}
	if in.ClearCategory {
		p.CategoryID = nil
	}

	if in.IntervalUnit != nil {
		p.IntervalUnit = *in.IntervalUnit
	}
	if in.IntervalCount != nil {
		p.IntervalCount = int32(*in.IntervalCount)
	}
	if in.StartOn != nil {
		if p.StartOn, err = parseDate("start_on", *in.StartOn); err != nil {
			return RecurringItem{}, err
		}
	}
	if in.TotalPayments != nil && in.ClearTotalPayments {
		return RecurringItem{}, Invalid("clear_total_payments", "cannot be combined with total_payments")
	}
	if in.TotalPayments != nil {
		n := int32(*in.TotalPayments)
		p.TotalPayments = &n
	}
	if in.ClearTotalPayments {
		p.TotalPayments = nil
	}
	var total *int
	if p.TotalPayments != nil {
		n := int(*p.TotalPayments)
		total = &n
	}
	if err := validateRecurringSchedule(p.IntervalUnit, int(p.IntervalCount), total); err != nil {
		return RecurringItem{}, err
	}

	if in.Status != nil {
		if !slices.Contains(recurringStatuses, *in.Status) {
			return RecurringItem{}, Invalid("status", "must be one of "+strings.Join(recurringStatuses, ", "))
		}
		p.Status = *in.Status
	}

	if err := s.q.UpdateRecurringItem(ctx, p); pgErrorCode(err) == pgUniqueViolation {
		return RecurringItem{}, s.recurringNameConflict(ctx, p.Name, id)
	} else if err != nil {
		return RecurringItem{}, err
	}
	return s.GetRecurringItem(ctx, id)
}

// RecurringSummary returns the committed monthly cost: active items only,
// each normalized with MonthlyAmount, grouped per currency and split into
// expenses and income. Currencies are never converted.
func (s *Service) RecurringSummary(ctx context.Context) (RecurringSummary, error) {
	active := StatusActive
	items, err := s.ListRecurringItems(ctx, RecurringFilter{Status: &active})
	if err != nil {
		return RecurringSummary{}, err
	}
	byCurrency := map[string]*RecurringCurrencySummary{}
	for _, it := range items {
		cs, ok := byCurrency[it.Currency]
		if !ok {
			cs = &RecurringCurrencySummary{Currency: it.Currency, MinorUnits: it.MinorUnits}
			byCurrency[it.Currency] = cs
		}
		if it.Type == TypeIncome {
			cs.Income += it.MonthlyAmount
			cs.IncomeCount++
		} else {
			cs.Expense += it.MonthlyAmount
			cs.ExpenseCount++
		}
	}
	out := RecurringSummary{Currencies: make([]RecurringCurrencySummary, 0, len(byCurrency))}
	for _, cs := range byCurrency {
		cs.Net = cs.Income - cs.Expense
		out.Currencies = append(out.Currencies, *cs)
	}
	sort.Slice(out.Currencies, func(i, j int) bool { return out.Currencies[i].Currency < out.Currencies[j].Currency })
	return out, nil
}
