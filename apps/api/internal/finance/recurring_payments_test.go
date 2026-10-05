package finance

import (
	"testing"
	"time"
)

func TestScheduleIsDueOn(t *testing.T) {
	t.Parallel()
	monthly := Schedule{UnitMonth, 1, day("2026-01-31"), nil}
	capped := Schedule{UnitMonth, 1, day("2026-01-15"), ptr(3)}
	tests := []struct {
		name string
		s    Schedule
		day  string
		want bool
	}{
		{"start", monthly, "2026-01-31", true},
		{"clamped february", monthly, "2026-02-28", true},
		{"not a due date", monthly, "2026-02-27", false},
		{"before start", monthly, "2025-12-31", false},
		{"last installment", capped, "2026-03-15", true},
		{"after last installment", capped, "2026-04-15", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.s.IsDueOn(day(tc.day)); got != tc.want {
				t.Fatalf("IsDueOn(%s) = %v, want %v", tc.day, got, tc.want)
			}
		})
	}
}

func TestScheduleLatestDueOnOrBefore(t *testing.T) {
	t.Parallel()
	monthly := Schedule{UnitMonth, 1, day("2026-01-15"), nil}
	capped := Schedule{UnitMonth, 1, day("2026-01-15"), ptr(3)}
	tests := []struct {
		name   string
		s      Schedule
		day    string
		want   string
		wantOK bool
	}{
		{"not started", monthly, "2026-01-14", "", false},
		{"on start", monthly, "2026-01-15", "2026-01-15", true},
		{"between", monthly, "2026-03-20", "2026-03-15", true},
		{"on a due date", monthly, "2026-03-15", "2026-03-15", true},
		{"exhausted keeps last", capped, "2027-01-01", "2026-03-15", true},
		{"exhausted on last", capped, "2026-03-15", "2026-03-15", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, ok := tc.s.LatestDueOnOrBefore(day(tc.day))
			if ok != tc.wantOK || (ok && got.Format(time.DateOnly) != tc.want) {
				t.Fatalf("got %v %v, want %s %v", got, ok, tc.want, tc.wantOK)
			}
		})
	}
}

func TestScheduleClosestDueOn(t *testing.T) {
	t.Parallel()
	monthly := Schedule{UnitMonth, 1, day("2026-01-10"), nil}
	capped := Schedule{UnitMonth, 1, day("2026-01-10"), ptr(2)}
	tests := []struct {
		name string
		s    Schedule
		day  string
		want string
	}{
		{"closer to previous", monthly, "2026-03-12", "2026-03-10"},
		{"closer to next", monthly, "2026-03-28", "2026-04-10"},
		{"tie picks earlier", monthly, "2026-03-25", "2026-03-10"},
		{"before start", monthly, "2025-06-01", "2026-01-10"},
		{"after the last installment", capped, "2027-06-01", "2026-02-10"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.s.ClosestDueOn(day(tc.day)).Format(time.DateOnly); got != tc.want {
				t.Fatalf("ClosestDueOn(%s) = %s, want %s", tc.day, got, tc.want)
			}
		})
	}
}

func TestCurrentPeriod(t *testing.T) {
	t.Parallel()
	monthly := Schedule{UnitMonth, 1, day("2026-01-10"), nil}
	capped := Schedule{UnitMonth, 1, day("2026-01-10"), ptr(2)}
	paidOn := func(dates ...string) func(time.Time) bool {
		return func(d time.Time) bool {
			for _, p := range dates {
				if d.Equal(day(p)) {
					return true
				}
			}
			return false
		}
	}
	tests := []struct {
		name       string
		s          Schedule
		itemStatus string
		today      string
		paid       func(time.Time) bool
		want       *RecurringPeriod
	}{
		{"paid", monthly, StatusActive, "2026-03-15", paidOn("2026-03-10"), &RecurringPeriod{"2026-03-10", PeriodPaid}},
		{"overdue the day after the due date", monthly, StatusActive, "2026-03-11", paidOn(), &RecurringPeriod{"2026-03-10", PeriodOverdue}},
		{"pending on the due date", monthly, StatusActive, "2026-03-10", paidOn(), &RecurringPeriod{"2026-03-10", PeriodPending}},
		{"not started is pending", monthly, StatusActive, "2025-12-01", paidOn(), &RecurringPeriod{"2026-01-10", PeriodPending}},
		{"not started but prepaid", monthly, StatusActive, "2025-12-01", paidOn("2026-01-10"), &RecurringPeriod{"2026-01-10", PeriodPaid}},
		{"older unpaid periods do not count", monthly, StatusActive, "2026-03-15", paidOn("2026-03-10"), &RecurringPeriod{"2026-03-10", PeriodPaid}},
		{"paused has no period", monthly, StatusPaused, "2026-03-15", paidOn(), nil},
		{"cancelled has no period", monthly, StatusCancelled, "2026-03-15", paidOn(), nil},
		{"exhausted and paid", capped, StatusActive, "2027-01-01", paidOn("2026-02-10"), &RecurringPeriod{"2026-02-10", PeriodPaid}},
		{"exhausted and unpaid", capped, StatusActive, "2027-01-01", paidOn("2026-01-10"), &RecurringPeriod{"2026-02-10", PeriodOverdue}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := CurrentPeriod(tc.s, tc.itemStatus, day(tc.today), tc.paid)
			switch {
			case tc.want == nil && got != nil:
				t.Fatalf("expected no period, got %+v", got)
			case tc.want != nil && (got == nil || *got != *tc.want):
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}
