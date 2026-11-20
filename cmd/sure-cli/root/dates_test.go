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
	// back, so a monthly bill is counted once per month. A window ending on
	// the last day of a month is whole calendar months: ending on Apr 30 it
	// must not reach back to Mar 31, a second month-end bill.
	cases := []struct {
		end    string
		months int
		want   string
	}{
		{"2026-11-20", 1, "2026-10-21"},
		{"2026-03-31", 1, "2026-03-01"}, // February has no 31st
		{"2026-03-30", 1, "2026-03-01"},
		{"2026-02-28", 1, "2026-02-01"}, // month end
		{"2026-04-30", 1, "2026-04-01"}, // month end after a 31-day month
		{"2026-06-30", 3, "2026-04-01"},
		{"2024-02-29", 1, "2024-02-01"},
		{"2026-12-31", 12, "2026-01-01"},
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

func TestParseISODate(t *testing.T) {
	for _, s := range []string{"", "2026-3-2", "2026-02-30", " 2026-02-03", "2026-02-03T00:00:00Z"} {
		if _, err := parseISODate("date", s); err == nil {
			t.Errorf("parseISODate(%q) accepted an invalid date", s)
		}
	}
	if d, err := parseISODate("date", "2024-02-29"); err != nil || d.Day() != 29 {
		t.Errorf("parseISODate(2024-02-29) = %v, %v", d, err)
	}
	if d, err := parseOptionalISODate("date", ""); err != nil || !d.IsZero() {
		t.Errorf("parseOptionalISODate(\"\") = %v, %v; want zero, nil", d, err)
	}
}
