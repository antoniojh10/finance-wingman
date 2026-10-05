package finance

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

var (
	detectToday = day("2026-10-05")
	acctA       = uuid.MustParse("00000000-0000-0000-0000-00000000000a")
	acctB       = uuid.MustParse("00000000-0000-0000-0000-00000000000b")
)

func tx(account uuid.UUID, typ, desc, date string, amount int64) candidate {
	return candidate{ID: uuid.New(), AccountID: account, Type: typ, Description: desc, OccurredOn: day(date), Amount: amount}
}

func expense(desc, date string, amount int64) candidate {
	return tx(acctA, TypeExpense, desc, date, amount)
}

func TestNormalizeDescription(t *testing.T) {
	t.Parallel()
	tests := map[string]string{
		"Netflix":                "netflix",
		"  NETFLIX  #4421 ":      "netflix",
		"Spotify*Premium 10/22":  "spotifypremium",
		"Pago   de   luz (CFE)":  "pago de luz cfe",
		"12345":                  "",
		"":                       "",
		"Gym-Membership 2026-10": "gymmembership",
	}
	for in, want := range tests {
		if got := normalizeDescription(in); got != want {
			t.Errorf("normalizeDescription(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDetectMonthlyWithSmallAmountChanges(t *testing.T) {
	t.Parallel()
	cands := []candidate{
		expense("Netflix #1", "2026-07-05", 18900),
		expense("NETFLIX 2", "2026-08-06", 19500),
		expense("netflix", "2026-09-04", 19900),
		expense("Netflix", "2026-10-05", 19900),
	}
	got := detectRecurring(cands, detectToday)
	if len(got) != 1 {
		t.Fatalf("expected one detection, got %d", len(got))
	}
	d := got[0]
	if d.Spec.name != FrequencyMonthly || len(d.Chain) != 4 || d.Amount != 19700 || d.Group.Description != "netflix" {
		t.Fatalf("unexpected detection: %+v", d)
	}
	if d.Confidence <= 0.5 || d.Confidence > 1 {
		t.Fatalf("unexpected confidence %v", d.Confidence)
	}
	if got := suggestedStartOn(d).Format(time.DateOnly); got != "2026-07-05" {
		t.Fatalf("start on = %s, want the median day (5th) of the first month", got)
	}
}

func TestDetectMinimumOccurrences(t *testing.T) {
	t.Parallel()
	monthly := []candidate{
		expense("Gym", "2026-08-10", 50000),
		expense("Gym", "2026-09-10", 50000),
	}
	if got := detectRecurring(monthly, detectToday); len(got) != 0 {
		t.Fatalf("2 monthly occurrences are not enough: %+v", got)
	}
	monthly = append(monthly, expense("Gym", "2026-10-10", 50000)) // future
	if got := detectRecurring(monthly, detectToday); len(got) != 0 {
		t.Fatalf("future transactions must be ignored: %+v", got)
	}
	monthly[2] = expense("Gym", "2026-07-10", 50000)
	if got := detectRecurring(monthly, detectToday); len(got) != 1 {
		t.Fatalf("3 monthly occurrences are enough, got %d", len(got))
	}
}

func TestDetectWeekly(t *testing.T) {
	t.Parallel()
	cands := []candidate{
		expense("Cleaner", "2026-09-14", 30000),
		expense("Cleaner", "2026-09-21", 30000),
		expense("Cleaner", "2026-09-29", 30000), // one day late
		expense("Cleaner", "2026-10-05", 30000), // one day early
	}
	got := detectRecurring(cands, detectToday)
	if len(got) != 1 || got[0].Spec.name != FrequencyWeekly || len(got[0].Chain) != 4 {
		t.Fatalf("expected a weekly detection: %+v", got)
	}
	// Three weeks are not enough.
	if got := detectRecurring(cands[1:], detectToday); len(got) != 0 {
		t.Fatalf("expected no detection with 3 occurrences: %+v", got)
	}
	// Two days off the period breaks the chain.
	late := append([]candidate(nil), cands...)
	late[2] = expense("Cleaner", "2026-09-30", 30000)
	if got := detectRecurring(late, detectToday); len(got) != 0 {
		t.Fatalf("a gap of 2 days off the period must break the chain: %+v", got)
	}
}

func TestDetectYearly(t *testing.T) {
	t.Parallel()
	cands := []candidate{
		expense("Domain renewal", "2024-09-22", 25000),
		expense("Domain renewal", "2025-09-20", 26000),
	}
	got := detectRecurring(cands, detectToday)
	if len(got) != 1 || got[0].Spec.name != FrequencyYearly {
		t.Fatalf("expected a yearly detection: %+v", got)
	}
	// Outside the 25-month lookback only one occurrence remains.
	old := []candidate{
		expense("Domain renewal", "2024-08-22", 25000),
		expense("Domain renewal", "2025-08-20", 26000),
	}
	if got := detectRecurring(old, detectToday); len(got) != 0 {
		t.Fatalf("occurrence outside the lookback must not count: %+v", got)
	}
}

func TestDetectIncome(t *testing.T) {
	t.Parallel()
	cands := []candidate{
		tx(acctA, TypeIncome, "Payroll", "2026-08-01", 3000000),
		tx(acctA, TypeIncome, "Payroll", "2026-09-01", 3000000),
		tx(acctA, TypeIncome, "Payroll", "2026-10-01", 3050000),
	}
	got := detectRecurring(cands, detectToday)
	if len(got) != 1 || got[0].Group.Type != TypeIncome {
		t.Fatalf("expected an income detection: %+v", got)
	}
}

func TestDetectIgnoresIrregularAndInvalid(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		cands []candidate
	}{
		{"irregular one-offs", []candidate{
			expense("Tacos", "2026-06-02", 15000),
			expense("Tacos", "2026-07-19", 15000),
			expense("Tacos", "2026-08-03", 15000),
			expense("Tacos", "2026-09-30", 15000),
		}},
		{"amounts too different", []candidate{
			expense("Dinner", "2026-07-05", 10000),
			expense("Dinner", "2026-08-05", 20000),
			expense("Dinner", "2026-09-05", 40000),
		}},
		{"date tolerance exceeded", []candidate{
			expense("Rent", "2026-07-01", 100000),
			expense("Rent", "2026-08-07", 100000),
			expense("Rent", "2026-09-14", 100000),
		}},
		{"stopped long ago", []candidate{
			expense("Old gym", "2026-02-10", 50000),
			expense("Old gym", "2026-03-10", 50000),
			expense("Old gym", "2026-04-10", 50000),
		}},
		{"no description", []candidate{
			expense("", "2026-07-05", 100), expense("  ", "2026-08-05", 100), expense("123", "2026-09-05", 100),
		}},
		{"transfers", []candidate{
			tx(acctA, TypeTransfer, "Savings", "2026-07-05", 100), tx(acctA, TypeTransfer, "Savings", "2026-08-05", 100),
			tx(acctA, TypeTransfer, "Savings", "2026-09-05", 100),
		}},
		{"split across accounts", []candidate{
			tx(acctA, TypeExpense, "Spotify", "2026-07-05", 100), tx(acctB, TypeExpense, "Spotify", "2026-08-05", 100),
			tx(acctA, TypeExpense, "Spotify", "2026-09-05", 100),
		}},
		{"split across types", []candidate{
			tx(acctA, TypeExpense, "Refund", "2026-07-05", 100), tx(acctA, TypeIncome, "Refund", "2026-08-05", 100),
			tx(acctA, TypeExpense, "Refund", "2026-09-05", 100),
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := detectRecurring(tc.cands, detectToday); len(got) != 0 {
				t.Fatalf("expected no detection, got %+v", got)
			}
		})
	}
}

func TestDetectSkipsNoiseInsideASeries(t *testing.T) {
	t.Parallel()
	cands := []candidate{
		expense("Spotify", "2026-07-05", 11500),
		expense("Spotify", "2026-07-20", 11500), // stray extra charge
		expense("Spotify", "2026-08-05", 11500),
		expense("Spotify", "2026-09-05", 11500),
		expense("Spotify", "2026-09-28", 90000), // outlier amount
	}
	got := detectRecurring(cands, detectToday)
	if len(got) != 1 || len(got[0].Chain) != 3 || got[0].Chain[0].OccurredOn != day("2026-07-05") {
		t.Fatalf("expected the 3 regular charges, got %+v", got)
	}
}

func TestDetectConfidenceAndOrdering(t *testing.T) {
	t.Parallel()
	steady := []candidate{
		expense("Netflix", "2026-04-05", 19900), expense("Netflix", "2026-05-05", 19900),
		expense("Netflix", "2026-06-05", 19900), expense("Netflix", "2026-07-05", 19900),
		expense("Netflix", "2026-08-05", 19900), expense("Netflix", "2026-09-05", 19900),
	}
	noisy := []candidate{
		expense("Water bill", "2026-07-02", 40000), expense("Water bill", "2026-08-06", 44000),
		expense("Water bill", "2026-09-02", 37000),
	}
	got := detectRecurring(append(noisy, steady...), detectToday)
	if len(got) != 2 || got[0].Group.Description != "netflix" || got[0].Confidence <= got[1].Confidence {
		t.Fatalf("steady series should rank first: %+v", got)
	}
	if got[0].Confidence != 1 {
		t.Fatalf("a perfect series with 2x the minimum scores 1, got %v", got[0].Confidence)
	}
	if got[1].Confidence <= 0 || got[1].Confidence >= 1 {
		t.Fatalf("unexpected confidence %v", got[1].Confidence)
	}
}

func TestMostCommonCategory(t *testing.T) {
	t.Parallel()
	a, b := uuid.New(), uuid.New()
	chain := []candidate{{CategoryID: &a}, {CategoryID: &b}, {CategoryID: &b}, {}}
	if got := mostCommonCategory(chain); got == nil || *got != b {
		t.Fatalf("got %v, want %v", got, b)
	}
	if got := mostCommonCategory([]candidate{{}, {}}); got != nil {
		t.Fatalf("expected no category, got %v", got)
	}
	tie := []candidate{{CategoryID: &a}, {CategoryID: &b}}
	if got := mostCommonCategory(tie); got == nil || *got != b {
		t.Fatalf("a tie goes to the most recent category, got %v", got)
	}
}

func TestNamesMatch(t *testing.T) {
	t.Parallel()
	tests := []struct {
		a, b string
		want bool
	}{
		{"netflix", "netflix", true},
		{"netflix com", "netflix", true},
		{"gym", "gym membership", true},
		{"ab", "abc", false},
		{"rent", "netflix", false},
		{"", "netflix", false},
	}
	for _, tc := range tests {
		if got := namesMatch(tc.a, tc.b); got != tc.want {
			t.Errorf("namesMatch(%q, %q) = %v, want %v", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestSuggestedStartOnStaysNearTheFirstOccurrence(t *testing.T) {
	t.Parallel()
	got := detectRecurring([]candidate{
		expense("Rent", "2026-08-01", 100000),
		expense("Rent", "2026-08-31", 100000),
		expense("Rent", "2026-09-30", 100000),
	}, detectToday)
	if len(got) != 1 {
		t.Fatalf("expected one detection: %+v", got)
	}
	start := suggestedStartOn(got[0])
	if start.Format(time.DateOnly) != "2026-07-30" {
		t.Fatalf("start on = %s, want 2026-07-30", start.Format(time.DateOnly))
	}
	sched := Schedule{Unit: UnitMonth, Count: 1, StartOn: start}
	seen := map[time.Time]bool{}
	for _, c := range got[0].Chain {
		seen[sched.ClosestDueOn(c.OccurredOn)] = true
	}
	if len(seen) != 3 {
		t.Fatalf("each occurrence should settle its own period: %v", seen)
	}
}
