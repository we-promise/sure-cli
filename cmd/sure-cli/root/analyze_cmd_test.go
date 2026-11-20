package root

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
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
		"min-count":  fmt.Sprint(analyze.DefaultLeakParams.MinCount),
		// Unset amount thresholds mean "scaled defaults", not 0.
		"min-total": "0",
		"max-avg":   "0",
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

func TestAnalyzeCommandsTakeNoArgs(t *testing.T) {
	for _, name := range []string{"leaks", "runway"} {
		sub := findSub(t, newAnalyzeCmd(), name)
		if sub.Args == nil || sub.Args(sub, []string{"extra"}) == nil {
			t.Fatalf("analyze %s must reject positional arguments", name)
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

func TestLeakParams(t *testing.T) {
	changed := func(set ...string) func(string) bool {
		return func(flag string) bool {
			for _, s := range set {
				if s == flag {
					return true
				}
			}
			return false
		}
	}
	cases := []struct {
		name               string
		set                []string
		minTotal, maxAvg   float64
		currency           string
		wantTotal, wantAvg float64
		wantCurrency       string
		wantErr            string
	}{
		{name: "defaults", wantTotal: 1500, wantAvg: 1000},
		{name: "currency is normalised", currency: " jpy ", wantTotal: 1500, wantAvg: 1000, wantCurrency: "JPY"},
		{name: "explicit EUR thresholds", set: []string{"min-total", "max-avg"}, minTotal: 20, maxAvg: 2.5, currency: "EUR", wantTotal: 2000, wantAvg: 250, wantCurrency: "EUR"},
		{name: "explicit JPY thresholds", set: []string{"min-total", "max-avg"}, minTotal: 3000, maxAvg: 800, currency: "JPY", wantTotal: 3000, wantAvg: 800, wantCurrency: "JPY"},
		{name: "explicit zero minimum total", set: []string{"min-total"}, minTotal: 0, currency: "EUR", wantTotal: 0, wantAvg: 1000, wantCurrency: "EUR"},
		{name: "min-total needs a currency", set: []string{"min-total"}, minTotal: 20, wantErr: "--min-total"},
		{name: "max-avg needs a currency", set: []string{"max-avg"}, maxAvg: 2, wantErr: "--max-avg"},
		{name: "zero max-avg", set: []string{"max-avg"}, maxAvg: 0, currency: "EUR", wantErr: "max_avg"},
		{name: "negative min-total", set: []string{"min-total"}, minTotal: -1, currency: "EUR", wantErr: "min_total"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p, err := leakParams(changed(c.set...), 3, c.minTotal, c.maxAvg, c.currency)
			if c.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), c.wantErr) {
					t.Fatalf("err = %v, want one naming %s", err, c.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if p.MinTotalMinor != c.wantTotal || p.MaxAvgMinor != c.wantAvg || p.Currency != c.wantCurrency || p.MinCount != 3 {
				t.Fatalf("params = %+v", p)
			}
		})
	}
	if _, err := leakParams(changed(), 0, 0, 0, ""); err == nil {
		t.Fatal("min-count 0 must be rejected")
	}
}

func TestLeaksWindow(t *testing.T) {
	now := time.Date(2026, 5, 20, 18, 30, 0, 0, time.UTC)
	date := func(s string) time.Time { d, _ := time.Parse(time.DateOnly, s); return d }

	cases := []struct {
		name               string
		months             int
		start, end         string
		wantStart, wantEnd string
		wantErr            bool
	}{
		// Each calendar day once: Feb 21..May 20, not Feb 20 too (same rule as status/export).
		{name: "months back from today", months: 3, wantStart: "2026-02-21", wantEnd: "2026-05-20"},
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
				t.Fatalf("window = %s..%s, want %s..%s", start.Format(time.DateOnly), end.Format(time.DateOnly), c.wantStart, c.wantEnd)
			}
		})
	}
}

func TestRunwayWindow(t *testing.T) {
	// Late in the day, and in a zone whose date is already tomorrow in UTC.
	now := time.Date(2026, 5, 20, 23, 30, 0, 0, time.FixedZone("UTC-2", -2*3600))
	start, end, err := runwayWindow(30, now)
	if err != nil {
		t.Fatal(err)
	}
	if start.Format(time.DateOnly) != "2026-04-22" || end.Format(time.DateOnly) != "2026-05-21" {
		t.Fatalf("window = %s..%s, want the 30 UTC days 2026-04-22..2026-05-21", start.Format(time.DateOnly), end.Format(time.DateOnly))
	}
	if got := analyze.InclusiveDays(start, end); got != 30 {
		t.Fatalf("window has %d days, want 30", got)
	}
	for _, days := range []int{0, -5} {
		if _, _, err := runwayWindow(days, now); err == nil {
			t.Fatalf("--days %d must be rejected", days)
		}
	}
}

// txJSON renders one transaction as Sure's _transaction.json.jbuilder does.
func txJSON(id, date, name, kind, currency, amount string, splitParent bool) string {
	return fmt.Sprintf(`{"id":%q,"date":%q,"amount":%q,"amount_decimal":%q,"currency":%q,"name":%q,`+
		`"classification":"expense","kind":%q,"excluded":%t,"split_parent":%t,"account":{"id":"a1","name":"Main"}}`,
		id, date, amount, amount, currency, name, kind, splitParent, splitParent)
}

// analyzeServer serves one page of transactions on the reportable account a1
// (the accounts list FetchTransactionsWindow filters by) and, for runway, the
// account itself.
func analyzeServer(t *testing.T, account string, txs ...string) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/v1/transactions":
			_, _ = fmt.Fprintf(w, `{"transactions":[%s],"pagination":{"page":1,"per_page":100,"total_pages":1}}`, strings.Join(txs, ","))
		case r.URL.Path == "/api/v1/accounts":
			_, _ = w.Write([]byte(`{"accounts":[{"id":"a1","name":"Main","currency":"EUR","exclude_from_reports":false}],"pagination":{"page":1,"per_page":100,"total_pages":1}}`))
		case strings.HasPrefix(r.URL.Path, "/api/v1/accounts/") && account != "":
			_, _ = w.Write([]byte(account))
		default:
			t.Errorf("unexpected request %s", r.URL.RequestURI())
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	apiClientFor(t, srv.URL)
}

// runAnalyze runs `analyze args...` and decodes the envelope's data into out.
func runAnalyze(t *testing.T, out any, args ...string) {
	t.Helper()
	cmd := newAnalyzeCmd()
	cmd.SetArgs(args)
	stdout := captureStdout(t, func() {
		format = "json"
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute: %v", err)
		}
	})
	var env struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("unmarshal: %v\nout=%q", err, stdout)
	}
	if err := json.Unmarshal(env.Data, out); err != nil {
		t.Fatalf("unmarshal data: %v\ndata=%s", err, env.Data)
	}
}

func TestAnalyzeLeaks_EchoesAppliedParamsAndSkipsUnreportableRows(t *testing.T) {
	analyzeServer(t, "",
		txJSON("c1", "2026-01-05", "Coffee", "standard", "EUR", "0.5", false),
		txJSON("c2", "2026-01-12", "Coffee", "standard", "EUR", "0.5", false),
		txJSON("c3", "2026-01-19", "Coffee", "standard", "EUR", "0.5", false),
		// Counted, these would make Coffee 5 rows averaging over 20 EUR: no leak.
		txJSON("split", "2026-01-20", "Coffee", "standard", "EUR", "100.0", true),
		txJSON("move", "2026-01-21", "Coffee", "funds_movement", "EUR", "0.5", false),
	)
	var data struct {
		Window     map[string]any          `json:"window"`
		Params     analyze.LeakParams      `json:"params"`
		Candidates []analyze.LeakCandidate `json:"candidates"`
	}
	runAnalyze(t, &data, "leaks", "--start-date", "2026-01-01", "--end-date", "2026-01-31", "--currency", "eur", "--min-total", "0")

	if data.Window["start"] != "2026-01-01" || data.Window["end"] != "2026-01-31" || data.Window["days"] != float64(31) {
		t.Fatalf("window = %#v", data.Window)
	}
	want := analyze.LeakParams{MinCount: 3, MinTotalMinor: 0, MaxAvgMinor: 1000, Currency: "EUR"}
	if data.Params != want {
		t.Fatalf("params = %+v, want the applied %+v", data.Params, want)
	}
	if len(data.Candidates) != 1 {
		t.Fatalf("candidates = %+v", data.Candidates)
	}
	c := data.Candidates[0]
	if c.Name != "Coffee" || c.Count != 3 || c.TotalAmount != 1.5 || c.AvgAmount != 0.5 {
		t.Fatalf("candidate = %+v, want the 3 reportable Coffee rows", c)
	}
}

func TestAnalyzeRunway_BurnSkipsUnreportableRows(t *testing.T) {
	today := time.Now().UTC().Format(time.DateOnly)
	analyzeServer(t, `{"id":"a1","balance":"€300.00","balance_cents":30000,"currency":"EUR"}`,
		txJSON("rent", today, "Rent", "standard", "EUR", "30.0", false),
		txJSON("split", today, "Groceries", "standard", "EUR", "1000.0", true),
		txJSON("move", today, "To savings", "funds_movement", "EUR", "500.0", false),
	)
	var s analyze.RunwaySummary
	runAnalyze(t, &s, "runway", "--account-id", "a1", "--days", "30")
	if s.Balance != 300 || s.WindowDays != 30 || s.AvgMonthlyBurn != 30 || s.RunwayMonths != 10 {
		t.Fatalf("runway = %+v, want balance 300, 30 days, burn 30 and 10 months", s)
	}
}
