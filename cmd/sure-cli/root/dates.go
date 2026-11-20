package root

import (
	"errors"
	"fmt"
	"time"
)

// parseISODate parses a required YYYY-MM-DD date given as flag.
func parseISODate(flag, s string) (time.Time, error) {
	t, err := time.Parse(time.DateOnly, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("%s: invalid date %q (expected YYYY-MM-DD)", flag, s)
	}
	return t, nil
}

// parseOptionalISODate accepts "" (flag unset, zero time) or a YYYY-MM-DD date.
func parseOptionalISODate(flag, s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	return parseISODate(flag, s)
}

// parseOptionalDateRange validates the conventional --start-date/--end-date
// pair: each is optional, but any value given must be an ISO date.
func parseOptionalDateRange(start, end string) (time.Time, time.Time, error) {
	s, err := parseOptionalISODate("start-date", start)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	e, err := parseOptionalISODate("end-date", end)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	return s, e, nil
}

// parsePairedDateRange validates a --start-date/--end-date pair that the
// server only accepts together: both or neither, ISO dates, start <= end.
// It reports whether a range was given.
func parsePairedDateRange(start, end string) (bool, error) {
	if start == "" && end == "" {
		return false, nil
	}
	if start == "" || end == "" {
		return false, errors.New("start-date and end-date must be given together")
	}
	s, e, err := parseOptionalDateRange(start, end)
	if err != nil {
		return false, err
	}
	if s.After(e) {
		return false, errors.New("start-date must be on or before end-date")
	}
	return true, nil
}

// today is now's calendar date in UTC, the day Sure's date filters use.
func today(now time.Time) time.Time {
	now = now.UTC()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
}

// lookbackWindow is the window of n units up to today, today included;
// back gives its first day (lookbackStart for months, daysBack for days).
// n is the value of --flag and must be positive.
func lookbackWindow(flag string, n int, back func(end time.Time, n int) time.Time, now time.Time) (time.Time, time.Time, error) {
	if n <= 0 {
		return time.Time{}, time.Time{}, fmt.Errorf("--%s must be positive", flag)
	}
	end := today(now)
	return back(end, n), end, nil
}

// daysBack is the first day of the window of days days that ends on end's date.
func daysBack(end time.Time, days int) time.Time {
	return end.AddDate(0, 0, 1-days)
}

// lookbackStart is the first day of the window that ends on end's date and
// spans months calendar months: the day after the same date months earlier,
// clamped to the end of a shorter month. Each day of the month falls in the
// window once per month, so a monthly bill is counted once per month.
func lookbackStart(end time.Time, months int) time.Time {
	y, m, d := end.Date()
	first := time.Date(y, m-time.Month(months), 1, 0, 0, 0, 0, end.Location())
	if last := first.AddDate(0, 1, -1).Day(); d > last {
		d = last
	}
	return time.Date(first.Year(), first.Month(), d+1, 0, 0, 0, 0, end.Location())
}
