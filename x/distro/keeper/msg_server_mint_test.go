package keeper

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAddMonths(t *testing.T) {
	d := func(y, m, day int) time.Time {
		return time.Date(y, time.Month(m), day, 0, 0, 0, 0, time.UTC)
	}
	tests := []struct {
		name   string
		start  time.Time
		months int
		want   time.Time
	}{
		// Normal cases — identical to AddDate for days ≤ 28.
		{"day 25 +12mo", d(2025, 4, 25), 12, d(2026, 4, 25)},
		{"day 22 +12mo", d(2025, 7, 22), 12, d(2026, 7, 22)},
		{"day 22 +0mo", d(2025, 7, 22), 0, d(2025, 7, 22)},
		{"year boundary", d(2025, 12, 15), 1, d(2026, 1, 15)},
		// Month-end clamping cases — differs from AddDate.
		{"jan31 +1mo clamps to feb28", d(2025, 1, 31), 1, d(2025, 2, 28)},
		{"jan31 +2mo no clamp", d(2025, 1, 31), 2, d(2025, 3, 31)},
		{"jan31 +3mo clamps to apr30", d(2025, 1, 31), 3, d(2025, 4, 30)},
		{"mar31 +1mo clamps to apr30", d(2025, 3, 31), 1, d(2025, 4, 30)},
		// Leap year.
		{"jan31 +1mo leap year clamps to feb29", d(2024, 1, 31), 1, d(2024, 2, 29)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := addMonths(tc.start, tc.months)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestMonthsBetween(t *testing.T) {
	d := func(y, m, day int) time.Time {
		return time.Date(y, time.Month(m), day, 0, 0, 0, 0, time.UTC)
	}
	tests := []struct {
		name  string
		start time.Time
		end   time.Time
		want  int
	}{
		// Normal cases — no month-end edge.
		{"same day", d(2025, 4, 25), d(2025, 4, 25), 0},
		{"one month exact", d(2025, 4, 25), d(2025, 5, 25), 1},
		{"12 months exact (mainnet halving)", d(2025, 4, 25), d(2026, 4, 25), 12},
		{"one day before 12 months", d(2025, 4, 25), d(2026, 4, 24), 11},
		{"end before start", d(2025, 4, 25), d(2025, 3, 25), -1},
		// Month-end clamping cases — GCA-13 fix.
		{"jan31 to feb28 = 1 month", d(2025, 1, 31), d(2025, 2, 28), 1},
		{"jan31 to feb29 leap = 1 month", d(2024, 1, 31), d(2024, 2, 29), 1},
		{"jan31 to feb15 = 0 months", d(2025, 1, 31), d(2025, 2, 15), 0},
		{"mar31 to apr30 = 1 month", d(2025, 3, 31), d(2025, 4, 30), 1},
		{"jan31 to mar31 = 2 months", d(2025, 1, 31), d(2025, 3, 31), 2},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := monthsBetween(tc.start, tc.end)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestHalvingPeriodLimit(t *testing.T) {
	const maxSupply = uint64(35_000_000_000_000_000)

	tests := []struct {
		name      string
		period    uint64
		wantLimit uint64
	}{
		{"period 0 returns 0", 0, 0},
		{"period 1 (no shift)", 1, maxSupply / 2},
		{"period 2", 2, maxSupply / 4},
		{"period 64 (max safe shift)", 64, maxSupply / (uint64(1) << 63) / 2},
		{"period 65 returns 0 (shift would be 64)", 65, 0},
		{"period 100 returns 0", 100, 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := halvingPeriodLimit(maxSupply, tc.period)
			require.Equal(t, tc.wantLimit, got)
		})
	}
}
