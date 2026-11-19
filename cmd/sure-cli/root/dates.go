package root

import (
	"fmt"
	"time"
)

const isoDateLayout = "2006-01-02"

// parseOptionalISODate accepts "" (flag unset, zero time) or a YYYY-MM-DD date.
func parseOptionalISODate(flag, s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	t, err := time.Parse(isoDateLayout, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("%s: invalid date %q (expected YYYY-MM-DD)", flag, s)
	}
	return t, nil
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
