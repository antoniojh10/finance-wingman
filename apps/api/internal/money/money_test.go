package money

import (
	"errors"
	"math"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		in    string
		units int
		want  int64
	}{
		{"0", 2, 0},
		{"12", 2, 1200},
		{"12.5", 2, 1250},
		{"12.50", 2, 1250},
		{"12.500", 2, 1250},
		{"0.01", 2, 1},
		{".5", 2, 50},
		{"-3.25", 2, -325},
		{"+7", 2, 700},
		{"12,75", 2, 1275},
		{" 42.10 ", 2, 4210},
		{"1500", 0, 1500},
		{"1.2345", 4, 12345},
		{"007.10", 2, 710},
	}
	for _, tc := range tests {
		got, err := Parse(tc.in, tc.units)
		if err != nil {
			t.Errorf("Parse(%q, %d) unexpected error: %v", tc.in, tc.units, err)
			continue
		}
		if got != tc.want {
			t.Errorf("Parse(%q, %d) = %d, want %d", tc.in, tc.units, got, tc.want)
		}
	}
}

func TestParseErrors(t *testing.T) {
	tests := []struct {
		in    string
		units int
		want  error
	}{
		{"", 2, ErrInvalidAmount},
		{"abc", 2, ErrInvalidAmount},
		{"1.2.3", 2, ErrInvalidAmount},
		{"1,000.50", 2, ErrInvalidAmount},
		{"12.", 2, ErrInvalidAmount},
		{"-", 2, ErrInvalidAmount},
		{".", 2, ErrInvalidAmount},
		{"1e5", 2, ErrInvalidAmount},
		{"12.345", 2, ErrTooPrecise},
		{"12.5", 0, ErrTooPrecise},
		{"99999999999999999999", 2, ErrOutOfRange},
	}
	for _, tc := range tests {
		if _, err := Parse(tc.in, tc.units); !errors.Is(err, tc.want) {
			t.Errorf("Parse(%q, %d) error = %v, want %v", tc.in, tc.units, err, tc.want)
		}
	}
}

func TestFromFloat(t *testing.T) {
	tests := []struct {
		in    float64
		units int
		want  int64
	}{
		{150, 2, 15000},
		{19.99, 2, 1999},
	}
	for _, tc := range tests {
		got, err := FromFloat(tc.in, tc.units)
		if err != nil || got != tc.want {
			t.Errorf("FromFloat(%v) = %d, %v; want %d", tc.in, got, err, tc.want)
		}
	}
	a, b := 0.1, 0.2 // variables force float64 arithmetic (constants are exact)
	if _, err := FromFloat(a+b, 2); !errors.Is(err, ErrTooPrecise) {
		t.Errorf("binary float artifacts should be rejected as too precise, got %v", err)
	}
	if _, err := FromFloat(math.NaN(), 2); !errors.Is(err, ErrInvalidAmount) {
		t.Errorf("NaN should be invalid, got %v", err)
	}
	if _, err := FromFloat(math.Inf(1), 2); !errors.Is(err, ErrInvalidAmount) {
		t.Errorf("Inf should be invalid, got %v", err)
	}
}

func TestFormat(t *testing.T) {
	tests := []struct {
		amount int64
		units  int
		want   string
	}{
		{0, 2, "0.00"},
		{1, 2, "0.01"},
		{1250, 2, "12.50"},
		{-325, 2, "-3.25"},
		{1500, 0, "1500"},
		{-1500, 0, "-1500"},
		{12345, 4, "1.2345"},
		{math.MinInt64, 2, "-92233720368547758.08"},
	}
	for _, tc := range tests {
		if got := Format(tc.amount, tc.units); got != tc.want {
			t.Errorf("Format(%d, %d) = %q, want %q", tc.amount, tc.units, got, tc.want)
		}
	}
}

func TestRoundTrip(t *testing.T) {
	for _, amount := range []int64{0, 1, 99, 100, 123456789, -42} {
		got, err := Parse(Format(amount, 2), 2)
		if err != nil || got != amount {
			t.Errorf("round trip %d -> %q -> %d (%v)", amount, Format(amount, 2), got, err)
		}
	}
}
