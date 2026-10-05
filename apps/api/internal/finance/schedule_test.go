package finance

import (
	"slices"
	"testing"
	"time"
)

func day(s string) time.Time {
	t, err := time.Parse(time.DateOnly, s)
	if err != nil {
		panic(err)
	}
	return t
}

func ptr[T any](v T) *T { return &v }

func formatDates(ds []time.Time) []string {
	out := []string{}
	for _, d := range ds {
		out = append(out, d.Format(time.DateOnly))
	}
	return out
}

func TestScheduleDueOn(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		s    Schedule
		n    int
		want string
	}{
		{"first is start", Schedule{UnitMonth, 1, day("2026-01-15"), nil}, 0, "2026-01-15"},
		{"weekly", Schedule{UnitWeek, 1, day("2026-10-05"), nil}, 3, "2026-10-26"},
		{"every 2 weeks", Schedule{UnitWeek, 2, day("2026-10-05"), nil}, 2, "2026-11-02"},
		{"month end clamps to feb", Schedule{UnitMonth, 1, day("2026-01-31"), nil}, 1, "2026-02-28"},
		{"month end recovers after feb", Schedule{UnitMonth, 1, day("2026-01-31"), nil}, 2, "2026-03-31"},
		{"leap february", Schedule{UnitMonth, 1, day("2028-01-31"), nil}, 1, "2028-02-29"},
		{"day 30 in april", Schedule{UnitMonth, 1, day("2026-03-30"), nil}, 1, "2026-04-30"},
		{"every 3 months", Schedule{UnitMonth, 3, day("2026-01-31"), nil}, 1, "2026-04-30"},
		{"every 13 months crosses year", Schedule{UnitMonth, 13, day("2026-01-10"), nil}, 1, "2027-02-10"},
		{"yearly", Schedule{UnitYear, 1, day("2026-03-15"), nil}, 2, "2028-03-15"},
		{"yearly leap day", Schedule{UnitYear, 1, day("2028-02-29"), nil}, 1, "2029-02-28"},
		{"yearly leap day to leap", Schedule{UnitYear, 1, day("2028-02-29"), nil}, 4, "2032-02-29"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.s.DueOn(tc.n).Format(time.DateOnly); got != tc.want {
				t.Fatalf("DueOn(%d) = %s, want %s", tc.n, got, tc.want)
			}
		})
	}
}

func TestScheduleNextDueOn(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		s      Schedule
		from   string
		want   string
		wantOK bool
	}{
		{"before start", Schedule{UnitMonth, 1, day("2026-11-01"), nil}, "2026-10-05", "2026-11-01", true},
		{"on start", Schedule{UnitMonth, 1, day("2026-11-01"), nil}, "2026-11-01", "2026-11-01", true},
		{"on a due date", Schedule{UnitMonth, 1, day("2026-01-15"), nil}, "2026-10-15", "2026-10-15", true},
		{"day after a due date", Schedule{UnitMonth, 1, day("2026-01-15"), nil}, "2026-10-16", "2026-11-15", true},
		{"month end clamp", Schedule{UnitMonth, 1, day("2026-01-31"), nil}, "2026-02-01", "2026-02-28", true},
		{"month end after clamp", Schedule{UnitMonth, 1, day("2026-01-31"), nil}, "2026-03-01", "2026-03-31", true},
		{"weekly", Schedule{UnitWeek, 1, day("2026-10-05"), nil}, "2026-10-14", "2026-10-19", true},
		{"every 2 weeks", Schedule{UnitWeek, 2, day("2026-10-05"), nil}, "2026-10-20", "2026-11-02", true},
		{"every 3 months", Schedule{UnitMonth, 3, day("2026-01-10"), nil}, "2026-05-01", "2026-07-10", true},
		{"yearly leap day", Schedule{UnitYear, 1, day("2024-02-29"), nil}, "2026-01-01", "2026-02-28", true},
		{"yearly after due", Schedule{UnitYear, 1, day("2024-06-01"), nil}, "2026-06-02", "2027-06-01", true},
		{"last installment still due", Schedule{UnitMonth, 1, day("2026-01-15"), ptr(3)}, "2026-03-15", "2026-03-15", true},
		{"installments exhausted", Schedule{UnitMonth, 1, day("2026-01-15"), ptr(3)}, "2026-03-16", "", false},
		{"single payment exhausted", Schedule{UnitMonth, 1, day("2026-01-15"), ptr(1)}, "2026-01-16", "", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, ok := tc.s.NextDueOn(day(tc.from))
			if ok != tc.wantOK {
				t.Fatalf("NextDueOn ok = %v, want %v", ok, tc.wantOK)
			}
			if ok && got.Format(time.DateOnly) != tc.want {
				t.Fatalf("NextDueOn = %s, want %s", got.Format(time.DateOnly), tc.want)
			}
		})
	}
}

func TestScheduleDueDatesBetween(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		s        Schedule
		from, to string
		want     []string
	}{
		{"weekly", Schedule{UnitWeek, 1, day("2026-10-05"), nil}, "2026-10-01", "2026-10-31",
			[]string{"2026-10-05", "2026-10-12", "2026-10-19", "2026-10-26"}},
		{"month end", Schedule{UnitMonth, 1, day("2026-01-31"), nil}, "2026-01-01", "2026-04-30",
			[]string{"2026-01-31", "2026-02-28", "2026-03-31", "2026-04-30"}},
		{"leap year", Schedule{UnitMonth, 1, day("2028-01-31"), nil}, "2028-02-01", "2028-02-29",
			[]string{"2028-02-29"}},
		{"inclusive bounds", Schedule{UnitMonth, 1, day("2026-01-15"), nil}, "2026-02-15", "2026-04-15",
			[]string{"2026-02-15", "2026-03-15", "2026-04-15"}},
		{"every 2 months", Schedule{UnitMonth, 2, day("2026-01-01"), nil}, "2026-01-01", "2026-12-31",
			[]string{"2026-01-01", "2026-03-01", "2026-05-01", "2026-07-01", "2026-09-01", "2026-11-01"}},
		{"yearly", Schedule{UnitYear, 1, day("2026-05-01"), nil}, "2026-01-01", "2028-12-31",
			[]string{"2026-05-01", "2027-05-01", "2028-05-01"}},
		{"total payments caps the range", Schedule{UnitMonth, 1, day("2026-01-15"), ptr(3)}, "2026-01-01", "2026-12-31",
			[]string{"2026-01-15", "2026-02-15", "2026-03-15"}},
		{"range after exhaustion", Schedule{UnitMonth, 1, day("2026-01-15"), ptr(3)}, "2026-04-01", "2026-12-31", []string{}},
		{"range before start", Schedule{UnitMonth, 1, day("2026-06-15"), nil}, "2026-01-01", "2026-05-31", []string{}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := formatDates(tc.s.DueDatesBetween(day(tc.from), day(tc.to)))
			if !slices.Equal(got, tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestScheduleLastDueOn(t *testing.T) {
	t.Parallel()
	if _, ok := (Schedule{UnitMonth, 1, day("2026-01-15"), nil}).LastDueOn(); ok {
		t.Fatal("open-ended schedule must have no last due date")
	}
	tests := []struct {
		name string
		s    Schedule
		want string
	}{
		{"one payment", Schedule{UnitMonth, 1, day("2026-01-15"), ptr(1)}, "2026-01-15"},
		{"12 months", Schedule{UnitMonth, 1, day("2026-01-31"), ptr(12)}, "2026-12-31"},
		{"3 months ending in february", Schedule{UnitMonth, 1, day("2026-12-31"), ptr(3)}, "2027-02-28"},
		{"every 2 weeks", Schedule{UnitWeek, 2, day("2026-10-05"), ptr(4)}, "2026-11-16"},
		{"yearly", Schedule{UnitYear, 1, day("2026-03-01"), ptr(3)}, "2028-03-01"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, ok := tc.s.LastDueOn()
			if !ok || got.Format(time.DateOnly) != tc.want {
				t.Fatalf("LastDueOn = %s (%v), want %s", got.Format(time.DateOnly), ok, tc.want)
			}
		})
	}
}

func TestMonthlyAmount(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		amount int64
		unit   string
		count  int
		want   int64
	}{
		{"monthly", 10000, UnitMonth, 1, 10000},
		{"every 3 months", 30000, UnitMonth, 3, 10000},
		{"every 3 months rounds half up", 10001, UnitMonth, 3, 3334},
		{"every 3 months rounds down", 10000, UnitMonth, 3, 3333},
		{"yearly", 120000, UnitYear, 1, 10000},
		{"yearly rounds", 100000, UnitYear, 1, 8333},
		{"yearly rounds half up", 100050, UnitYear, 1, 8338},
		{"every 2 years", 240000, UnitYear, 2, 10000},
		{"weekly", 1200, UnitWeek, 1, 5200},
		{"weekly rounds", 1000, UnitWeek, 1, 4333},
		{"every 2 weeks", 1200, UnitWeek, 2, 2600},
		{"tiny amount rounds to zero", 1, UnitYear, 1, 0},
		{"max amount does not overflow", maxRecurringAmount, UnitWeek, 1, 4333333333333333},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := MonthlyAmount(tc.amount, tc.unit, tc.count); got != tc.want {
				t.Fatalf("MonthlyAmount = %d, want %d", got, tc.want)
			}
		})
	}
}
