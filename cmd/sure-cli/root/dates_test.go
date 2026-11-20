package root

import (
	"testing"
	"time"
)

func TestLookbackStart(t *testing.T) {
	day := func(s string) time.Time {
		d, err := time.Parse(time.DateOnly, s)
		if err != nil {
			t.Fatal(err)
		}
		return d
	}
	// The window [start, end] covers each day of the month once per month
	// back, so a monthly bill is counted once per month.
	cases := []struct {
		end    string
		months int
		want   string
	}{
		{"2026-11-20", 1, "2026-10-21"},
		{"2026-03-31", 1, "2026-03-01"}, // February has no 31st
		{"2026-03-30", 1, "2026-03-01"},
		{"2026-02-28", 1, "2026-01-29"},
		{"2024-03-30", 1, "2024-03-01"}, // leap February ends on the 29th
		{"2026-01-15", 1, "2025-12-16"},
		{"2026-11-20", 3, "2026-08-21"},
		{"2026-11-20", 12, "2025-11-21"},
	}
	for _, c := range cases {
		end := day(c.end).Add(15 * time.Hour)
		if got := lookbackStart(end, c.months).Format(time.DateOnly); got != c.want {
			t.Errorf("lookbackStart(%s, %d) = %s, want %s", c.end, c.months, got, c.want)
		}
	}
}
