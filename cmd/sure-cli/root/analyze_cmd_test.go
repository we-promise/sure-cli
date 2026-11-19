package root

import (
	"strconv"
	"testing"
	"time"

	"github.com/we-promise/sure-cli/internal/analyze"
)

func TestAnalyzeLeaksFlags(t *testing.T) {
	leaks := findSub(t, newAnalyzeCmd(), "leaks")
	defaults := map[string]string{
		"months":     "3",
		"start-date": "",
		"end-date":   "",
		"currency":   "",
		"min-count":  strconv.Itoa(analyze.DefaultLeakParams.MinCount),
		"min-total":  strconv.FormatFloat(analyze.DefaultLeakParams.MinTotal, 'g', -1, 64),
		"max-avg":    strconv.FormatFloat(analyze.DefaultLeakParams.MaxAvg, 'g', -1, 64),
	}
	for name, want := range defaults {
		f := leaks.Flags().Lookup(name)
		if f == nil {
			t.Fatalf("missing --%s", name)
		}
		if f.DefValue != want {
			t.Fatalf("--%s default = %q, want %q", name, f.DefValue, want)
		}
	}
}

func TestAnalyzeRunwayFlags(t *testing.T) {
	runway := findSub(t, newAnalyzeCmd(), "runway")
	for _, name := range []string{"account-id", "days"} {
		if runway.Flags().Lookup(name) == nil {
			t.Fatalf("missing --%s", name)
		}
	}
}

func TestLeaksWindow(t *testing.T) {
	now := time.Date(2026, 5, 20, 18, 30, 0, 0, time.UTC)
	date := func(s string) time.Time { d, _ := time.Parse(isoDateLayout, s); return d }

	cases := []struct {
		name               string
		months             int
		start, end         string
		wantStart, wantEnd string
		wantErr            bool
	}{
		{name: "months back from today", months: 3, wantStart: "2026-02-20", wantEnd: "2026-05-20"},
		{name: "explicit range wins over months", months: 3, start: "2026-01-01", end: "2026-01-31", wantStart: "2026-01-01", wantEnd: "2026-01-31"},
		{name: "start without end", months: 3, start: "2026-01-01", wantErr: true},
		{name: "reversed range", months: 3, start: "2026-02-01", end: "2026-01-01", wantErr: true},
		{name: "bad date", months: 3, start: "2026-1-1", end: "2026-01-31", wantErr: true},
		{name: "no months", months: 0, wantErr: true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			start, end, err := leaksWindow(c.months, c.start, c.end, now)
			if c.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got %v..%v", start, end)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !start.Equal(date(c.wantStart)) || !end.Equal(date(c.wantEnd)) {
				t.Fatalf("window = %s..%s, want %s..%s", start.Format(isoDateLayout), end.Format(isoDateLayout), c.wantStart, c.wantEnd)
			}
		})
	}
}
