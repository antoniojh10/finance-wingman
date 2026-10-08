package finance

import "testing"

func TestMedianAmount(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		values []int64
		want   int64
	}{
		{"one month", []int64{500}, 500},
		{"two months average", []int64{100, 201}, 151},
		{"two months even", []int64{100, 200}, 150},
		{"three months", []int64{300, 100, 200}, 200},
		{"zero months count", []int64{0, 0, 900}, 0},
		{"outlier ignored", []int64{100, 120, 100000}, 120},
		{"all zero", []int64{0, 0, 0}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := medianAmount(tt.values); got != tt.want {
				t.Fatalf("medianAmount(%v) = %d, want %d", tt.values, got, tt.want)
			}
		})
	}
}

func TestMedianAmountKeepsInput(t *testing.T) {
	t.Parallel()
	values := []int64{3, 1, 2}
	medianAmount(values)
	if values[0] != 3 || values[1] != 1 || values[2] != 2 {
		t.Fatalf("input was reordered: %v", values)
	}
}

func TestRoundUpToUnit(t *testing.T) {
	t.Parallel()
	tests := []struct {
		amount int64
		units  int
		want   int64
	}{
		{12345, 2, 12400},
		{12300, 2, 12300},
		{1, 2, 100},
		{0, 2, 0},
		{1501, 0, 1501},
		{1501, 3, 2000},
	}
	for _, tt := range tests {
		if got := roundUpToUnit(tt.amount, tt.units); got != tt.want {
			t.Errorf("roundUpToUnit(%d, %d) = %d, want %d", tt.amount, tt.units, got, tt.want)
		}
	}
}
