package root

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/we-promise/sure-cli/internal/api"
	errs "github.com/we-promise/sure-cli/internal/errors"
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
		failed bool
	}{
		{"family zone", http.StatusOK, `{"timezone":"Pacific/Auckland"}`, "Pacific/Auckland", false},
		// Sure resolves a family without a zone to its own default, UTC.
		{"no zone set", http.StatusOK, `{"timezone":null}`, "UTC", false},
		{"blank zone", http.StatusOK, `{"timezone":""}`, "UTC", false},
		// Families saved before Sure stored IANA names keep a Rails name,
		// which ActiveSupport::TimeZone maps.
		{"rails name", http.StatusOK, `{"timezone":"Madrid"}`, "Europe/Madrid", false},
		{"rails name with spaces", http.StatusOK, `{"timezone":"Eastern Time (US & Canada)"}`, "America/New_York", false},
		// A name ActiveSupport does not know is UTC in Sure.
		{"unknown zone", http.StatusOK, `{"timezone":"Nowhere/Atlantis"}`, "UTC", false},
		{"Go-only name", http.StatusOK, `{"timezone":"Local"}`, "UTC", false},
		// A server without the endpoint, or a failed request: the user's
		// own zone is the best guess left, and the failure is reported.
		{"endpoint missing", http.StatusNotFound, `{"error":"not_found"}`, time.Local.String(), true},
		{"server error", http.StatusInternalServerError, `{}`, time.Local.String(), true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			client := familySettingsServer(t, c.status, c.body)
			loc, ce := familyLocation(client)
			if loc.String() != c.want {
				t.Fatalf("familyLocation = %s, want %s", loc, c.want)
			}
			if (ce != nil) != c.failed {
				t.Fatalf("error = %v, want failed=%t", ce, c.failed)
			}
		})
	}
}

func TestRailsTimeZones_EveryZoneLoads(t *testing.T) {
	for name, iana := range railsTimeZones {
		if _, err := time.LoadLocation(iana); err != nil {
			t.Errorf("%s -> %s: %v", name, iana, err)
		}
	}
	if n := len(railsTimeZones); n != 152 {
		t.Fatalf("%d zones, want the 152 of ActiveSupport::TimeZone::MAPPING", n)
	}
}

// warnedEnvelope runs fn and returns the envelope it prints.
func warnedEnvelope(t *testing.T, fn func()) (data map[string]any, warnings []map[string]any) {
	t.Helper()
	var env struct {
		Data map[string]any `json:"data"`
		Meta struct {
			Warnings []map[string]any `json:"warnings"`
		} `json:"meta"`
	}
	out := captureStdout(t, fn)
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	return env.Data, env.Meta.Warnings
}

func TestCreateDateDefaultsToTheFamilysToday(t *testing.T) {
	pinClock(t, testNow)
	kiritimati, _ := time.LoadLocation("Pacific/Kiritimati") // UTC+14
	for _, c := range []struct {
		cmd  func() *cobra.Command
		args []string
		date func(map[string]any) any
	}{
		{newTransactionsCmd, []string{"create", "--account-id", "a1", "--name", "x", "--amount", "1"}, func(d map[string]any) any {
			return d["request"].(map[string]any)["body"].(map[string]any)["transaction"].(map[string]any)["date"]
		}},
		{newValuationsCmd, []string{"create", "--account-id", "a1", "--amount", "1"}, func(d map[string]any) any {
			return d["request"].(map[string]any)["body"].(map[string]any)["valuation"].(map[string]any)["date"]
		}},
		{newTradesCmd, []string{"create", "--account-id", "a1", "--type", "deposit", "--amount", "1"}, func(d map[string]any) any {
			return d["request"].(map[string]any)["body"].(map[string]any)["trade"].(map[string]any)["date"]
		}},
	} {
		t.Run(c.cmd().Name(), func(t *testing.T) {
			familySettingsServer(t, http.StatusOK, `{"timezone":"Pacific/Kiritimati"}`)
			cmd := c.cmd()
			cmd.SetArgs(c.args)
			data, warnings := warnedEnvelope(t, func() { format = "json"; _ = cmd.Execute() })
			if got, want := c.date(data), testNow.In(kiritimati).Format(time.DateOnly); got != want {
				t.Fatalf("date = %v, want %s", got, want)
			}
			if len(warnings) != 0 {
				t.Fatalf("warnings = %v", warnings)
			}

			// With --date there is nothing to look up.
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				t.Errorf("unexpected request %s", r.URL.Path)
			}))
			t.Cleanup(srv.Close)
			apiClientFor(t, srv.URL)
			cmd = c.cmd()
			cmd.SetArgs(append(c.args, "--date", "2026-01-02"))
			data, _ = warnedEnvelope(t, func() { _ = cmd.Execute() })
			if got := c.date(data); got != "2026-01-02" {
				t.Fatalf("date = %v, want the flag's", got)
			}
		})
	}
}

func TestFailedTimeZoneLookupIsAWarning(t *testing.T) {
	pinClock(t, testNow)
	familySettingsServer(t, http.StatusNotFound, `{"error":"not_found"}`)
	cmd := newTransactionsCmd()
	cmd.SetArgs([]string{"create", "--account-id", "a1", "--name", "x", "--amount", "1"})
	data, warnings := warnedEnvelope(t, func() { format = "json"; _ = cmd.Execute() })
	if len(warnings) != 1 || warnings[0]["code"] != errs.CodeNotFound {
		t.Fatalf("warnings = %v, want the failed lookup as not_found", warnings)
	}
	date := data["request"].(map[string]any)["body"].(map[string]any)["transaction"].(map[string]any)["date"]
	if want := testNow.Local().Format(time.DateOnly); date != want {
		t.Fatalf("date = %v, want the local %s", date, want)
	}
}
