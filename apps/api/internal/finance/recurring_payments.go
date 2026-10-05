package finance

import (
	"context"
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/store"
)

// Period statuses. A period is one due date of a recurring item.
const (
	PeriodPaid    = "paid"
	PeriodPending = "pending"
	PeriodOverdue = "overdue"
)

// RecurringPeriod is the state of one due date of a recurring item.
type RecurringPeriod struct {
	DueOn  string `json:"due_on" format:"date"`
	Status string `json:"status" enum:"paid,pending,overdue" doc:"paid: a linked transaction exists for this due date. overdue: the due date is before today and it is not paid. pending: otherwise"`
}

// RecurringPayment is the most recent transaction linked to an item.
type RecurringPayment struct {
	TransactionID uuid.UUID `json:"transaction_id"`
	Date          string    `json:"date" format:"date" doc:"Date of the transaction"`
	Amount        int64     `json:"amount" doc:"Amount actually paid, in minor units"`
	DueOn         string    `json:"due_on" format:"date" doc:"Due date (period) the payment settles"`
}

// UpcomingRecurring is one due date of an active item in the upcoming list.
type UpcomingRecurring struct {
	Item   RecurringItem `json:"item"`
	DueOn  string        `json:"due_on" format:"date"`
	Status string        `json:"status" enum:"paid,pending,overdue"`
}

// RecurringPaymentInput registers a payment of a recurring item.
type RecurringPaymentInput struct {
	Period string `json:"period,omitempty" format:"date" doc:"Due date being paid; must be a due date of the item. Defaults to the due date closest to date"`
	Date   string `json:"date,omitempty" format:"date" doc:"Date of the transaction. Defaults to today"`
	Amount *int64 `json:"amount,omitempty" minimum:"1" doc:"Amount actually paid, in minor units. Defaults to the item's estimate, which is never changed by a payment"`
}

// LinkRecurringInput links an existing transaction to a recurring item.
type LinkRecurringInput struct {
	RecurringID uuid.UUID `json:"recurring_id" format:"uuid"`
	Period      string    `json:"period,omitempty" format:"date" doc:"Due date being paid; must be a due date of the item. Defaults to the due date closest to the transaction date"`
}

func (r RecurringItem) schedule() Schedule {
	start, _ := time.Parse(dateLayout, r.StartOn)
	return Schedule{Unit: r.IntervalUnit, Count: r.IntervalCount, StartOn: start, TotalPayments: r.TotalPayments}
}

// periodStatus classifies one due date. Overdue starts the day after the
// due date (no grace period).
func periodStatus(due, today time.Time, paid bool) string {
	switch {
	case paid:
		return PeriodPaid
	case due.Before(today):
		return PeriodOverdue
	default:
		return PeriodPending
	}
}

// CurrentPeriod returns the period an active item is currently in: the
// latest due date on or before today, or the first one when the schedule
// has not started. Paused and cancelled items have no current period.
// isPaid tells whether a due date has a linked transaction.
func CurrentPeriod(s Schedule, itemStatus string, today time.Time, isPaid func(time.Time) bool) *RecurringPeriod {
	if itemStatus != StatusActive {
		return nil
	}
	due, ok := s.LatestDueOnOrBefore(today)
	if !ok {
		due = s.StartOn
	}
	return &RecurringPeriod{DueOn: formatDate(due), Status: periodStatus(due, today, isPaid(due))}
}

type paidSet map[uuid.UUID]map[time.Time]bool

func (p paidSet) paid(id uuid.UUID) func(time.Time) bool {
	return func(d time.Time) bool { return p[id][d] }
}

// enrichRecurring fills current_period and last_payment from linked
// transactions and returns the paid periods for further use.
func (s *Service) enrichRecurring(ctx context.Context, items []RecurringItem) (paidSet, error) {
	paid := paidSet{}
	if len(items) == 0 {
		return paid, nil
	}
	ids := make([]uuid.UUID, len(items))
	for i, it := range items {
		ids[i] = it.ID
	}
	periods, err := s.q.ListRecurringPaidPeriods(ctx, ids)
	if err != nil {
		return nil, err
	}
	for _, p := range periods {
		if p.RecurringID == nil || p.RecurringDueOn == nil {
			continue
		}
		if paid[*p.RecurringID] == nil {
			paid[*p.RecurringID] = map[time.Time]bool{}
		}
		paid[*p.RecurringID][*p.RecurringDueOn] = true
	}
	last, err := s.q.ListRecurringLastPayments(ctx, ids)
	if err != nil {
		return nil, err
	}
	lastByItem := map[uuid.UUID]store.ListRecurringLastPaymentsRow{}
	for _, l := range last {
		if l.RecurringID != nil {
			lastByItem[*l.RecurringID] = l
		}
	}
	today := s.Today()
	for i := range items {
		it := &items[i]
		it.CurrentPeriod = CurrentPeriod(it.schedule(), it.Status, today, paid.paid(it.ID))
		if l, ok := lastByItem[it.ID]; ok && l.RecurringDueOn != nil {
			it.LastPayment = &RecurringPayment{
				TransactionID: l.ID, Date: formatDate(l.OccurredOn), Amount: l.Amount, DueOn: formatDate(*l.RecurringDueOn),
			}
		}
	}
	return paid, nil
}

// UpcomingRecurring lists the due dates of active items in the next days
// (today included), plus the current period of items that are overdue,
// ordered by due date.
func (s *Service) UpcomingRecurring(ctx context.Context, days int) ([]UpcomingRecurring, error) {
	if days < 1 || days > 366 {
		return nil, Invalid("days", "must be between 1 and 366")
	}
	active := StatusActive
	items, err := s.ListRecurringItems(ctx, RecurringFilter{Status: &active})
	if err != nil {
		return nil, err
	}
	paid, err := s.enrichRecurring(ctx, items)
	if err != nil {
		return nil, err
	}
	today := s.Today()
	to := today.AddDate(0, 0, days)
	out := []UpcomingRecurring{}
	for _, it := range items {
		var dues []time.Time
		if it.CurrentPeriod != nil && it.CurrentPeriod.Status == PeriodOverdue {
			if d, err := time.Parse(dateLayout, it.CurrentPeriod.DueOn); err == nil {
				dues = append(dues, d)
			}
		}
		dues = append(dues, it.schedule().DueDatesBetween(today, to)...)
		for _, due := range dues {
			out = append(out, UpcomingRecurring{
				Item: it, DueOn: formatDate(due), Status: periodStatus(due, today, paid[it.ID][due]),
			})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].DueOn < out[j].DueOn })
	return out, nil
}

// resolvePeriod validates an explicit period or picks the due date closest
// to the transaction date.
func resolvePeriod(item RecurringItem, period string, date time.Time) (time.Time, error) {
	sched := item.schedule()
	if period == "" {
		return sched.ClosestDueOn(date), nil
	}
	due, err := parseDate("period", period)
	if err != nil {
		return time.Time{}, err
	}
	if !sched.IsDueOn(due) {
		return time.Time{}, Invalid("period", "is not a due date of this recurring item (due dates follow start_on, interval_unit, interval_count and total_payments)")
	}
	return due, nil
}

// PeriodPaidError is returned by RegisterRecurringPayment with
// RejectIfPaid when the target period already has a linked transaction.
type PeriodPaidError struct {
	Conflict *Error
	DueOn    string
	// NextDueOn is the following due date, empty when the schedule ends.
	NextDueOn string
}

func (e *PeriodPaidError) Error() string { return e.Conflict.Error() }

func (e *PeriodPaidError) Unwrap() error { return e.Conflict }

// PaymentOption changes how RegisterRecurringPayment behaves.
type PaymentOption func(*paymentOptions)

type paymentOptions struct{ rejectIfPaid bool }

// RejectIfPaid makes RegisterRecurringPayment fail with a PeriodPaidError
// (a conflict) when the target period is already paid, instead of
// recording another payment for it. Callers that let the period default
// use it to avoid accidental double charges.
func RejectIfPaid() PaymentOption { return func(o *paymentOptions) { o.rejectIfPaid = true } }

// RegisterRecurringPayment creates the transaction that pays one period of
// an active recurring item (type, account, category and description come
// from the item) and links it, atomically. The item's estimate is not
// changed by the amount actually paid.
func (s *Service) RegisterRecurringPayment(ctx context.Context, itemID uuid.UUID, in RecurringPaymentInput, opts ...PaymentOption) (Transaction, error) {
	var o paymentOptions
	for _, opt := range opts {
		opt(&o)
	}
	item, err := s.GetRecurringItem(ctx, itemID)
	if err != nil {
		return Transaction{}, err
	}
	if item.Status != StatusActive {
		return Transaction{}, Conflict("recurring item is " + item.Status + ": set it to active before registering payments")
	}
	date := s.Today()
	if in.Date != "" {
		if date, err = parseDate("date", in.Date); err != nil {
			return Transaction{}, err
		}
	}
	period, err := resolvePeriod(item, in.Period, date)
	if err != nil {
		return Transaction{}, err
	}
	if o.rejectIfPaid {
		periods, err := s.q.ListRecurringPaidPeriods(ctx, []uuid.UUID{itemID})
		if err != nil {
			return Transaction{}, err
		}
		for _, p := range periods {
			if p.RecurringDueOn != nil && p.RecurringDueOn.Equal(period) {
				due := formatDate(period)
				paidErr := &PeriodPaidError{Conflict: Conflict("recurring item is already paid for " + due), DueOn: due}
				if next, ok := item.schedule().NextDueOn(period.AddDate(0, 0, 1)); ok {
					paidErr.NextDueOn = formatDate(next)
				}
				return Transaction{}, paidErr
			}
		}
	}
	amount := item.Amount
	if in.Amount != nil {
		amount = *in.Amount
	}

	var created Transaction
	err = s.withTx(ctx, func(tx *Service) error {
		var err error
		created, err = tx.CreateTransaction(ctx, TransactionInput{
			Type:        item.Type,
			AccountID:   item.AccountID,
			Amount:      amount,
			CategoryID:  item.CategoryID,
			Description: item.Name,
			OccurredOn:  formatDate(date),
		})
		if err != nil {
			return err
		}
		if err := tx.q.LinkTransactionToRecurring(ctx, store.LinkTransactionToRecurringParams{
			ID: created.ID, RecurringID: &itemID, RecurringDueOn: &period,
		}); err != nil {
			return err
		}
		created, err = tx.GetTransaction(ctx, created.ID)
		return err
	})
	return created, err
}

// LinkTransactionToRecurring links an existing transaction to a recurring
// item. The transaction must be of the item's type and use its account.
// A transaction that is already linked is re-linked to the new item and
// period. Several transactions may be linked to the same period.
func (s *Service) LinkTransactionToRecurring(ctx context.Context, txID uuid.UUID, in LinkRecurringInput) (Transaction, error) {
	rec, err := s.q.GetTransactionRecord(ctx, txID)
	if isNoRows(err) {
		return Transaction{}, NotFound("transaction")
	}
	if err != nil {
		return Transaction{}, err
	}
	row, err := s.q.GetRecurringItem(ctx, in.RecurringID)
	if isNoRows(err) {
		return Transaction{}, Invalid("recurring_id", "recurring item not found")
	}
	if err != nil {
		return Transaction{}, err
	}
	item := s.recurringFromRow(row)
	if rec.Type != item.Type {
		return Transaction{}, Invalid("recurring_id", "transaction type "+rec.Type+" does not match recurring item type "+item.Type)
	}
	if rec.AccountID != item.AccountID {
		return Transaction{}, Invalid("recurring_id", "transaction account differs from the recurring item account "+item.AccountName)
	}
	period, err := resolvePeriod(item, in.Period, rec.OccurredOn)
	if err != nil {
		return Transaction{}, err
	}
	if err := s.q.LinkTransactionToRecurring(ctx, store.LinkTransactionToRecurringParams{
		ID: txID, RecurringID: &in.RecurringID, RecurringDueOn: &period,
	}); err != nil {
		return Transaction{}, err
	}
	return s.GetTransaction(ctx, txID)
}

// UnlinkTransactionFromRecurring removes the link of a transaction. It is
// a no-op for a transaction that is not linked.
func (s *Service) UnlinkTransactionFromRecurring(ctx context.Context, txID uuid.UUID) (Transaction, error) {
	if _, err := s.GetTransaction(ctx, txID); err != nil {
		return Transaction{}, err
	}
	if err := s.q.UnlinkTransactionFromRecurring(ctx, txID); err != nil {
		return Transaction{}, err
	}
	return s.GetTransaction(ctx, txID)
}
