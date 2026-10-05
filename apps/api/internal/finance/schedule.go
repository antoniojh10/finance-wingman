package finance

import "time"

// Interval units supported by recurring items.
const (
	UnitWeek  = "week"
	UnitMonth = "month"
	UnitYear  = "year"
)

// Schedule describes when a recurring item is due: every Count units
// starting at StartOn, optionally limited to TotalPayments due dates
// (installments). All dates are calendar dates (UTC midnight) and the
// helpers are pure functions, so they need no database.
//
// Due dates are always derived from StartOn, never from the previous due
// date, so a monthly item that starts on the 31st is due on Jan 31, Feb 28
// (or 29), Mar 31, ... instead of drifting to the 28th after February.
type Schedule struct {
	Unit          string
	Count         int
	StartOn       time.Time
	TotalPayments *int
}

// DueOn returns the n-th due date (n = 0 is StartOn). It does not check
// TotalPayments.
func (s Schedule) DueOn(n int) time.Time {
	switch s.Unit {
	case UnitWeek:
		return s.StartOn.AddDate(0, 0, 7*s.Count*n)
	case UnitYear:
		return addMonthsClamped(s.StartOn, 12*s.Count*n)
	default:
		return addMonthsClamped(s.StartOn, s.Count*n)
	}
}

// addMonthsClamped adds months to t, clamping day 29-31 to the last day of
// the target month.
func addMonthsClamped(t time.Time, months int) time.Time {
	first := time.Date(t.Year(), t.Month()+time.Month(months), 1, 0, 0, 0, 0, time.UTC)
	lastDay := first.AddDate(0, 1, -1).Day()
	day := min(t.Day(), lastDay)
	return time.Date(first.Year(), first.Month(), day, 0, 0, 0, 0, time.UTC)
}

func (s Schedule) exhausted(n int) bool {
	return s.TotalPayments != nil && n >= *s.TotalPayments
}

// LastDueOn returns the final due date of an installment plan:
// StartOn + (TotalPayments-1) intervals. It reports false when the
// schedule has no total.
func (s Schedule) LastDueOn() (time.Time, bool) {
	if s.TotalPayments == nil {
		return time.Time{}, false
	}
	return s.DueOn(*s.TotalPayments - 1), true
}

// firstIndexOnOrAfter returns the smallest n such that DueOn(n) >= day,
// ignoring TotalPayments.
func (s Schedule) firstIndexOnOrAfter(day time.Time) int {
	if !day.After(s.StartOn) {
		return 0
	}
	var n int
	switch s.Unit {
	case UnitWeek:
		days := int(day.Sub(s.StartOn).Hours() / 24)
		n = days / (7 * s.Count)
	default:
		months := (day.Year()-s.StartOn.Year())*12 + int(day.Month()-s.StartOn.Month())
		step := s.Count
		if s.Unit == UnitYear {
			step = 12 * s.Count
		}
		n = months / step
	}
	// Back off once in case the estimate overshoots, then walk forward.
	if n > 0 {
		n--
	}
	for s.DueOn(n).Before(day) {
		n++
	}
	return n
}

// NextDueOn returns the first due date on or after day. It reports false
// when the schedule is already exhausted by then (all TotalPayments dates
// are before day).
func (s Schedule) NextDueOn(day time.Time) (time.Time, bool) {
	n := s.firstIndexOnOrAfter(day)
	if s.exhausted(n) {
		return time.Time{}, false
	}
	return s.DueOn(n), true
}

// DueDatesBetween returns every due date in [from, to], in order.
func (s Schedule) DueDatesBetween(from, to time.Time) []time.Time {
	dates := []time.Time{}
	for n := s.firstIndexOnOrAfter(from); !s.exhausted(n); n++ {
		d := s.DueOn(n)
		if d.After(to) {
			break
		}
		dates = append(dates, d)
	}
	return dates
}

// MonthlyAmount normalizes amount (minor units, charged once per interval)
// to an average monthly amount in minor units:
//
//	every N weeks:  amount * 52 / 12 / N
//	every N months: amount / N
//	every N years:  amount / 12 / N
//
// The result is rounded half up to the nearest minor unit. Each item is
// rounded on its own before items are added together, so the committed
// total is the sum of the monthly amounts shown per item. amount must be
// at most maxRecurringAmount so the intermediate product cannot overflow.
func MonthlyAmount(amount int64, unit string, count int) int64 {
	num, den := int64(1), int64(count)
	switch unit {
	case UnitWeek:
		num, den = 52, 12*int64(count)
	case UnitYear:
		den = 12 * int64(count)
	}
	return (amount*num + den/2) / den
}

// IsDueOn reports whether day is one of the schedule's due dates.
func (s Schedule) IsDueOn(day time.Time) bool {
	n := s.firstIndexOnOrAfter(day)
	return !s.exhausted(n) && s.DueOn(n).Equal(day)
}

// LatestDueOnOrBefore returns the last due date that is on or before day.
// It reports false when the schedule has not started yet.
func (s Schedule) LatestDueOnOrBefore(day time.Time) (time.Time, bool) {
	n := s.firstIndexOnOrAfter(day)
	if !s.DueOn(n).Equal(day) {
		n--
	}
	if s.TotalPayments != nil {
		n = min(n, *s.TotalPayments-1)
	}
	if n < 0 {
		return time.Time{}, false
	}
	return s.DueOn(n), true
}

// ClosestDueOn returns the due date nearest to day; on a tie the earlier
// one wins. Only real due dates are considered (never past TotalPayments).
func (s Schedule) ClosestDueOn(day time.Time) time.Time {
	next, hasNext := s.NextDueOn(day)
	prev, hasPrev := s.LatestDueOnOrBefore(day)
	switch {
	case !hasNext:
		return prev
	case !hasPrev:
		return next
	case next.Sub(day) < day.Sub(prev):
		return next
	default:
		return prev
	}
}
