// Package money converts between decimal amounts and integer minor units.
//
// Amounts are stored as int64 minor units (e.g. cents) to avoid floating
// point errors. The number of minor units depends on the currency.
package money

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

var (
	ErrInvalidAmount = errors.New("invalid amount")
	ErrTooPrecise    = errors.New("amount has more decimal places than the currency allows")
	ErrOutOfRange    = errors.New("amount out of range")
)

// Parse converts a decimal string such as "1234.5" or "-12.30" into minor
// units for a currency with the given number of decimal places. A comma is
// accepted as decimal separator when no dot is present ("12,50").
// Thousands separators are not supported.
func Parse(s string, minorUnits int) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, ErrInvalidAmount
	}
	if !strings.Contains(s, ".") && strings.Count(s, ",") == 1 {
		s = strings.Replace(s, ",", ".", 1)
	}

	negative := false
	switch s[0] {
	case '-':
		negative = true
		s = s[1:]
	case '+':
		s = s[1:]
	}

	whole, frac, hasDot := strings.Cut(s, ".")
	if whole == "" && frac == "" {
		return 0, ErrInvalidAmount
	}
	if hasDot && frac == "" {
		return 0, ErrInvalidAmount
	}
	if !isDigits(whole) || !isDigits(frac) {
		return 0, ErrInvalidAmount
	}

	frac = strings.TrimRight(frac, "0")
	if len(frac) > minorUnits {
		return 0, ErrTooPrecise
	}
	frac += strings.Repeat("0", minorUnits-len(frac))

	digits := strings.TrimLeft(whole+frac, "0")
	if digits == "" {
		return 0, nil
	}
	value, err := strconv.ParseInt(digits, 10, 64)
	if err != nil {
		return 0, ErrOutOfRange
	}
	if negative {
		value = -value
	}
	return value, nil
}

// FromFloat converts a float (as received from JSON) into minor units,
// rejecting values with more precision than the currency allows.
func FromFloat(f float64, minorUnits int) (int64, error) {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return 0, ErrInvalidAmount
	}
	// Formatting with the shortest representation avoids binary artifacts
	// such as 0.1 + 0.2 = 0.30000000000000004.
	return Parse(strconv.FormatFloat(f, 'f', -1, 64), minorUnits)
}

// Format renders minor units as a plain decimal string ("1234.50").
func Format(amount int64, minorUnits int) string {
	sign := ""
	// Use uint64 so math.MinInt64 does not overflow when negated.
	abs := uint64(amount)
	if amount < 0 {
		sign = "-"
		abs = uint64(-(amount + 1)) + 1
	}
	if minorUnits == 0 {
		return sign + strconv.FormatUint(abs, 10)
	}
	scale := uint64(math.Pow10(minorUnits))
	return fmt.Sprintf("%s%d.%0*d", sign, abs/scale, minorUnits, abs%scale)
}

func isDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
