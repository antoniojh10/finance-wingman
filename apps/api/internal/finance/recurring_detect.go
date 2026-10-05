package finance

import (
	"math"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
)

// Detection frequencies reported on suggestions.
const (
	FrequencyWeekly  = "weekly"
	FrequencyMonthly = "monthly"
	FrequencyYearly  = "yearly"
)

// amountTolerancePercent is how far an amount may be from the median of
// its group and still be considered the same charge.
const amountTolerancePercent = 15

// frequencySpec holds the detection rules of one frequency: the minimum
// number of occurrences within the lookback window and how far a gap may
// deviate (in days) from the exact period.
type frequencySpec struct {
	name           string
	unit           string
	minOccurrences int
	lookbackMonths int
	toleranceDays  int
	step           func(time.Time) time.Time
}

var frequencySpecs = []frequencySpec{
	{FrequencyWeekly, UnitWeek, 4, 2, 1, func(t time.Time) time.Time { return t.AddDate(0, 0, 7) }},
	{FrequencyMonthly, UnitMonth, 3, 6, 4, func(t time.Time) time.Time { return addMonthsClamped(t, 1) }},
	{FrequencyYearly, UnitYear, 2, 25, 7, func(t time.Time) time.Time { return addMonthsClamped(t, 12) }},
}

// maxLookbackMonths is the widest lookback of all frequencies.
const maxLookbackMonths = 25

// candidate is an unlinked expense or income considered by the heuristic.
type candidate struct {
	ID          uuid.UUID
	AccountID   uuid.UUID
	Type        string
	CategoryID  *uuid.UUID
	Description string
	OccurredOn  time.Time
	Amount      int64
}

// groupKey identifies the transactions that may form one recurring item.
type groupKey struct {
	AccountID   uuid.UUID
	Type        string
	Description string // normalized
}

// detection is a recurring pattern found in one group.
type detection struct {
	Group      groupKey
	Spec       frequencySpec
	Chain      []candidate // oldest first
	Amount     int64       // median amount of the chain
	Confidence float64
}

// normalizeDescription lowercases a description and strips digits and
// symbols, keeping letters; whitespace is collapsed. Two descriptions that
// normalize to the same non-empty text belong to the same group.
func normalizeDescription(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		switch {
		case unicode.IsLetter(r):
			b.WriteRune(r)
		case unicode.IsSpace(r):
			b.WriteRune(' ')
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

func medianAmount(amounts []int64) int64 {
	s := append([]int64(nil), amounts...)
	sort.Slice(s, func(i, j int) bool { return s[i] < s[j] })
	n := len(s)
	if n%2 == 1 {
		return s[n/2]
	}
	return (s[n/2-1] + s[n/2] + 1) / 2
}

func withinAmountTolerance(amount, median int64) bool {
	diff := amount - median
	if diff < 0 {
		diff = -diff
	}
	return diff*100 <= median*amountTolerancePercent
}

func daysBetween(a, b time.Time) int {
	return int(math.Round(b.Sub(a).Hours() / 24))
}

func absInt(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// detectRecurring finds regular patterns among unlinked expenses and
// incomes. For each group (account + type + normalized description) and
// each frequency it keeps the transactions inside the lookback window
// whose amount is within +-15% of the group's median, then looks for the
// longest chain in which each occurrence follows the previous one by one
// period, give or take the frequency's date tolerance. A chain becomes a
// detection when it reaches the minimum occurrences and is still alive:
// the next occurrence after the last one may be missing for at most one
// period plus the tolerance. When several frequencies match the same group
// the most confident one wins. Results are ordered by confidence.
func detectRecurring(cands []candidate, today time.Time) []detection {
	groups := map[groupKey][]candidate{}
	for _, c := range cands {
		if c.OccurredOn.After(today) || (c.Type != TypeExpense && c.Type != TypeIncome) {
			continue
		}
		norm := normalizeDescription(c.Description)
		if norm == "" {
			continue
		}
		k := groupKey{AccountID: c.AccountID, Type: c.Type, Description: norm}
		groups[k] = append(groups[k], c)
	}

	var out []detection
	for key, members := range groups {
		sort.Slice(members, func(i, j int) bool {
			if !members[i].OccurredOn.Equal(members[j].OccurredOn) {
				return members[i].OccurredOn.Before(members[j].OccurredOn)
			}
			return members[i].ID.String() < members[j].ID.String()
		})
		var best *detection
		for _, spec := range frequencySpecs {
			d, ok := detectFrequency(key, members, spec, today)
			if ok && (best == nil || d.Confidence > best.Confidence) {
				best = &d
			}
		}
		if best != nil {
			out = append(out, *best)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Confidence != out[j].Confidence {
			return out[i].Confidence > out[j].Confidence
		}
		if out[i].Group.Description != out[j].Group.Description {
			return out[i].Group.Description < out[j].Group.Description
		}
		return out[i].Group.AccountID.String() < out[j].Group.AccountID.String()
	})
	return out
}

func detectFrequency(key groupKey, members []candidate, spec frequencySpec, today time.Time) (detection, bool) {
	from := today.AddDate(0, -spec.lookbackMonths, 0)
	var window []candidate
	var amounts []int64
	for _, m := range members {
		if !m.OccurredOn.Before(from) {
			window = append(window, m)
			amounts = append(amounts, m.Amount)
		}
	}
	if len(window) < spec.minOccurrences {
		return detection{}, false
	}
	median := medianAmount(amounts)
	kept := window[:0:0]
	for _, m := range window {
		if withinAmountTolerance(m.Amount, median) {
			kept = append(kept, m)
		}
	}
	if len(kept) < spec.minOccurrences {
		return detection{}, false
	}

	// Longest chain (dynamic programming over the date-ordered members).
	n := len(kept)
	length := make([]int, n)
	prev := make([]int, n)
	for j := range kept {
		length[j], prev[j] = 1, -1
		bestDev := 0
		for i := 0; i < j; i++ {
			dev := absInt(daysBetween(spec.step(kept[i].OccurredOn), kept[j].OccurredOn))
			if dev > spec.toleranceDays {
				continue
			}
			if length[i]+1 > length[j] || (length[i]+1 == length[j] && dev < bestDev) {
				length[j], prev[j], bestDev = length[i]+1, i, dev
			}
		}
	}
	end := 0
	for j := range kept {
		if length[j] >= length[end] {
			end = j
		}
	}
	if length[end] < spec.minOccurrences {
		return detection{}, false
	}
	chain := make([]candidate, length[end])
	for j, k := end, length[end]-1; j >= 0; j, k = prev[j], k-1 {
		chain[k] = kept[j]
	}

	last := chain[len(chain)-1].OccurredOn
	if spec.step(spec.step(last)).AddDate(0, 0, spec.toleranceDays).Before(today) {
		return detection{}, false // stopped: more than a period overdue
	}

	chainAmounts := make([]int64, len(chain))
	for i, c := range chain {
		chainAmounts[i] = c.Amount
	}
	amount := medianAmount(chainAmounts)
	return detection{
		Group:      key,
		Spec:       spec,
		Chain:      chain,
		Amount:     amount,
		Confidence: confidence(chain, amount, spec),
	}, true
}

// confidence scores a chain from 0 to 1, rounded to two decimals:
//
//	0.4 * occurrences  min(1, n / (2 * minimum))  (the minimum alone scores 0.5)
//	0.3 * regularity   1 - mean gap deviation / date tolerance
//	0.3 * amount       1 - mean amount deviation / 15%
func confidence(chain []candidate, amount int64, spec frequencySpec) float64 {
	n := len(chain)
	count := math.Min(1, float64(n)/float64(2*spec.minOccurrences))

	var gapDev float64
	for i := 1; i < n; i++ {
		gapDev += float64(absInt(daysBetween(spec.step(chain[i-1].OccurredOn), chain[i].OccurredOn)))
	}
	regularity := 1 - gapDev/float64(n-1)/float64(spec.toleranceDays)

	var amountDev float64
	for _, c := range chain {
		amountDev += math.Abs(float64(c.Amount-amount)) / float64(amount)
	}
	amountScore := 1 - amountDev/float64(n)/(float64(amountTolerancePercent)/100)

	score := 0.4*count + 0.3*math.Max(0, regularity) + 0.3*math.Max(0, amountScore)
	return math.Round(math.Min(1, math.Max(0, score))*100) / 100
}

// mostCommonCategory returns the category used most often in the chain
// (the most recent one wins ties), or nil when none is categorized.
func mostCommonCategory(chain []candidate) *uuid.UUID {
	counts := map[uuid.UUID]int{}
	var best *uuid.UUID
	for i := len(chain) - 1; i >= 0; i-- {
		id := chain[i].CategoryID
		if id == nil {
			continue
		}
		counts[*id]++
		if best == nil || counts[*id] > counts[*best] {
			best = id
		}
	}
	return best
}

// suggestedStartOn picks the first due date of the suggested item: the
// oldest occurrence, with the day of month set to the median day for
// monthly series so the predicted due dates track the typical charge day.
// Weekly and yearly series start on the oldest occurrence.
func suggestedStartOn(d detection) time.Time {
	first := d.Chain[0].OccurredOn
	if d.Spec.unit != UnitMonth {
		return first
	}
	days := make([]int64, len(d.Chain))
	for i, c := range d.Chain {
		days[i] = int64(c.OccurredOn.Day())
	}
	// The month closest to the first occurrence, so that occurrence settles
	// the first period (a median day of 30 and a first charge on the 1st
	// start on the 30th of the previous month).
	medianDay := int(medianAmount(days))
	var start time.Time
	for offset := -1; offset <= 1; offset++ {
		monthStart := time.Date(first.Year(), first.Month()+time.Month(offset), 1, 0, 0, 0, 0, time.UTC)
		lastDay := monthStart.AddDate(0, 1, -1).Day()
		cand := monthStart.AddDate(0, 0, min(medianDay, lastDay)-1)
		if start.IsZero() || absInt(daysBetween(cand, first)) < absInt(daysBetween(start, first)) {
			start = cand
		}
	}
	return start
}
