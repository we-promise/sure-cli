package root

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/we-promise/sure-cli/internal/api"
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

func TestToday_IsTheDateWhereNowIs(t *testing.T) {
	akl, err := time.LoadLocation("Pacific/Auckland")
	if err != nil {
		t.Fatal(err)
	}
	la, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		now  time.Time
		want string
	}{
		{time.Date(2026, 11, 21, 9, 0, 0, 0, akl), "2026-11-21"}, // still Nov 20 in UTC
		{time.Date(2026, 11, 20, 18, 0, 0, 0, la), "2026-11-20"}, // already Nov 21 in UTC
	} {
		got := today(c.now)
		if got.Format(time.DateOnly) != c.want || got.Location() != time.UTC || got.Hour() != 0 {
			t.Errorf("today(%v) = %v, want %s at midnight UTC", c.now, got, c.want)
		}
	}
}

// familySettingsServer answers GET /api/v1/family_settings with status and body.
func familySettingsServer(t *testing.T, status int, body string) *api.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/family_settings" {
			t.Errorf("unexpected request %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		fmt.Fprint(w, body)
	}))
	t.Cleanup(srv.Close)
	return apiClientFor(t, srv.URL)
}

func TestFamilyLocation(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
		want   string
	}{
		{"family zone", http.StatusOK, `{"timezone":"Pacific/Auckland"}`, "Pacific/Auckland"},
		// Sure resolves a family without a zone to its own default, UTC.
		{"no zone set", http.StatusOK, `{"timezone":null}`, "UTC"},
		{"blank zone", http.StatusOK, `{"timezone":""}`, "UTC"},
		// A server without the endpoint, or a name Go cannot load: the
		// user's own zone is the best guess left.
		{"endpoint missing", http.StatusNotFound, `{"error":"not_found"}`, time.Local.String()},
		{"unloadable zone", http.StatusOK, `{"timezone":"Nowhere/Atlantis"}`, time.Local.String()},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			client := familySettingsServer(t, c.status, c.body)
			if got := familyLocation(client).String(); got != c.want {
				t.Fatalf("familyLocation = %s, want %s", got, c.want)
			}
		})
	}
}
