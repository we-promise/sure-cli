package root

import (
	"errors"
	"fmt"
	"maps"
	"time"
	// Family zones load on systems without a zoneinfo database (Windows).
	_ "time/tzdata"

	"github.com/we-promise/sure-cli/internal/api"
	errs "github.com/we-promise/sure-cli/internal/errors"
	"github.com/we-promise/sure-cli/internal/models"
)

// parseISODate parses a required YYYY-MM-DD date given as flag.
func parseISODate(flag, s string) (time.Time, error) {
	t, err := time.Parse(time.DateOnly, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("--%s: invalid date %q (expected YYYY-MM-DD)", flag, s)
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
		return false, errors.New("--start-date and --end-date must be given together")
	}
	s, e, err := parseOptionalDateRange(start, end)
	if err != nil {
		return false, err
	}
	if s.After(e) {
		return false, errors.New("--start-date must be on or before --end-date")
	}
	return true, nil
}

// today is now's calendar date where now is, as a midnight UTC date like
// those parseISODate returns.
func today(now time.Time) time.Time { return models.CalendarDate(now) }

// familyClock is the time now in the family's time zone. Sure dates entries
// and resolves "today" in that zone (Localize#switch_timezone), so a window
// that ends today must end on the family's date, not the UTC one. Each call
// reads the zone with one request, so a command calls it once and only when
// it needs today. When the zone cannot be read, the local zone stands in and
// the envelope says so under meta.warnings.
func familyClock(client *api.Client) func() time.Time {
	return func() time.Time {
		loc, ce := familyLocation(client)
		if ce != nil {
			details := map[string]any{"time_zone": loc.String()}
			maps.Copy(details, ce.Details)
			warn(errs.New(ce.Code, fmt.Sprintf("cannot read the family's time zone (%s); dates use this machine's zone, %s", ce.Message, loc)).WithDetails(details))
		}
		return time.Now().In(loc)
	}
}

// familyDateUsage describes a --date flag that defaults to familyToday.
const familyDateUsage = "date YYYY-MM-DD (default: today in the family's time zone, read from Sure)"

// familyToday is the family's date today as YYYY-MM-DD: the default of a
// --date flag left out.
func familyToday(client *api.Client) string {
	return today(familyClock(client)()).Format(time.DateOnly)
}

// familyLocation resolves the zone GET /api/v1/family_settings reports as
// Sure's Family#resolved_time_zone does: a Rails name through
// railsTimeZones, and UTC, Sure's default zone, for a family without one or
// with a name ActiveSupport::TimeZone does not know. A failed request (e.g.
// a server without the endpoint) returns the local zone with its error.
func familyLocation(client *api.Client) (*time.Location, *errs.CLIError) {
	var settings struct {
		Timezone string `json:"timezone"`
	}
	if ce := api.CheckResponse(client.Get("/api/v1/family_settings", &settings)); ce != nil {
		return time.Local, ce
	}
	name := settings.Timezone
	if iana, ok := railsTimeZones[name]; ok {
		name = iana
	}
	// "" and "Local" load in Go but are no zone name to TZInfo.
	if name == "" || name == "Local" {
		return time.UTC, nil
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return time.UTC, nil
	}
	return loc, nil
}

// lookbackWindow is the window of n units up to today, today included;
// back gives its first day (lookbackStart for months, daysBack for days).
// n is the value of --flag and must be positive. now is read only once n is
// valid, so an invalid flag fails before any request.
func lookbackWindow(flag string, n int, back func(end time.Time, n int) time.Time, now func() time.Time) (time.Time, time.Time, error) {
	if n <= 0 {
		return time.Time{}, time.Time{}, fmt.Errorf("--%s must be positive", flag)
	}
	end := today(now())
	return back(end, n), end, nil
}

// daysBack is the first day of the window of days days that ends on end's date.
func daysBack(end time.Time, days int) time.Time {
	return end.AddDate(0, 0, 1-days)
}

// lookbackStart is the first day of the window that ends on end's date and
// spans months calendar months: the day after the same date months earlier,
// clamped to the end of a shorter month. Each day of the month falls in the
// window once per month, so a monthly bill is counted once per month. A
// window ending on the last day of a month is whole calendar months: from
// Apr 30 it starts on Apr 1, not on Mar 31, the day after Mar 30.
func lookbackStart(end time.Time, months int) time.Time {
	y, m, d := end.Date()
	if end.AddDate(0, 0, 1).Month() != m {
		return time.Date(y, m-time.Month(months-1), 1, 0, 0, 0, 0, end.Location())
	}
	first := time.Date(y, m-time.Month(months), 1, 0, 0, 0, 0, end.Location())
	if last := first.AddDate(0, 1, -1).Day(); d > last {
		d = last
	}
	return time.Date(first.Year(), first.Month(), d+1, 0, 0, 0, 0, end.Location())
}
