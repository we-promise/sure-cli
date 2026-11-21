package root

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestAmountFromAPIOK_CentsPreferredOverFormatted(t *testing.T) {
	// Cents wins even when a formatted string is also present, so a CLI
	// upgrade that starts sending *_cents doesn't silently round through the
	// string path.
	m := map[string]any{
		"balance":       "€999.99",      // would parse to 999.99
		"balance_cents": float64(12345), // wins → 123.45
	}
	got, ok := amountFromAPIOK(m, "balance", "balance_cents")
	if !ok {
		t.Fatal("expected ok=true when cents present")
	}
	if got != 123.45 {
		t.Fatalf("expected cents path 123.45, got %v", got)
	}
}

func TestAmountFromAPIOK_CentsTypes(t *testing.T) {
	cases := []struct {
		name  string
		cents any
		want  float64
	}{
		{"float64", float64(12345), 123.45},  // JSON numbers decode as float64
		{"int", int(12345), 123.45},          // hand-built map literal in tests
		{"int64", int64(12345), 123.45},      // some HTTP libs use int64
		{"string-numeric", "12345", 123.45},  // some APIs emit cents as strings
		{"string-negative", "-2050", -20.50}, // signed cents
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := map[string]any{"balance_cents": c.cents}
			got, ok := amountFromAPIOK(m, "balance", "balance_cents")
			if !ok {
				t.Fatalf("expected ok=true for %s", c.name)
			}
			if got != c.want {
				t.Fatalf("%s: got %v want %v", c.name, got, c.want)
			}
		})
	}
}

func TestAmountFromAPIOK_CentsHonourCurrencyMinorUnit(t *testing.T) {
	// Sure emits balance_cents as amount * minor_unit_conversion, which is
	// not 100 for every currency (config/currencies.yml).
	cases := []struct {
		currency string
		cents    float64
		want     float64
	}{
		{"EUR", 123456, 1234.56},
		{"JPY", 1500, 1500},
		{"KWD", 12345, 12.345},
	}
	for _, c := range cases {
		m := map[string]any{"balance_cents": c.cents, "currency": c.currency}
		got, ok := amountFromAPIOK(m, "balance", "balance_cents")
		if !ok || got != c.want {
			t.Fatalf("%s: got %v ok=%v, want %v", c.currency, got, ok, c.want)
		}
	}
}

func TestSummarizeAccounts_SplitsByClassificationAndCurrency(t *testing.T) {
	accounts := []map[string]any{
		{"name": "Checking", "account_type": "depository", "classification": "asset", "currency": "EUR",
			"balance": "1.000,00 €", "balance_cents": float64(100000), "cash_balance_cents": float64(100000)},
		{"name": "Brokerage", "account_type": "investment", "classification": "asset", "currency": "EUR",
			"balance_cents": float64(500000), "cash_balance_cents": float64(20000)},
		{"name": "Visa", "account_type": "credit_card", "classification": "liability", "currency": "EUR",
			"balance_cents": float64(30000)},
		{"name": "US Checking", "account_type": "depository", "classification": "asset", "currency": "USD",
			"balance_cents": float64(25000), "cash_balance_cents": float64(25000)},
		{"name": "Yen wallet", "account_type": "depository", "classification": "asset", "currency": "JPY",
			"balance_cents": float64(5000), "cash_balance_cents": float64(5000)},
		// Excluded from reports: listed, but must not count towards any total.
		{"name": "Shared house", "account_type": "property", "classification": "asset", "currency": "EUR",
			"balance_cents": float64(9900000), "exclude_from_reports": true},
		{"name": "Old savings", "account_type": "depository", "classification": "asset", "currency": "EUR",
			"balance_cents": float64(70000), "cash_balance_cents": float64(70000), "exclude_from_reports": true},
	}

	totals, list := summarizeAccounts(accounts)

	if len(list) != len(accounts) {
		t.Fatalf("list has %d entries, want %d", len(list), len(accounts))
	}
	if list[5]["exclude_from_reports"] != true || list[2]["classification"] != "liability" {
		t.Fatalf("list must pass classification/exclude_from_reports through: %#v", list)
	}

	eur := totals["EUR"]
	if eur == nil {
		t.Fatalf("missing EUR totals: %#v", totals)
	}
	if eur.Assets != 6000 || eur.Liabilities != 300 || eur.NetWorth() != 5700 || eur.Cash != 1000 {
		t.Fatalf("EUR totals = %+v (net %v)", *eur, eur.NetWorth())
	}
	if usd := totals["USD"]; usd == nil || usd.Assets != 250 || usd.Cash != 250 || usd.Liabilities != 0 {
		t.Fatalf("USD totals = %+v", usd)
	}
	if jpy := totals["JPY"]; jpy == nil || jpy.Assets != 5000 {
		t.Fatalf("JPY totals = %+v", jpy)
	}
}

func TestParseBalanceSheet(t *testing.T) {
	// Money#as_json renders BigDecimal amounts as strings.
	res := map[string]any{
		"currency":    "EUR",
		"net_worth":   map[string]any{"amount": "5700.0", "currency": "EUR", "formatted": "€5.700,00"},
		"assets":      map[string]any{"amount": "6000.0", "currency": "EUR", "formatted": "€6.000,00"},
		"liabilities": map[string]any{"amount": float64(300), "currency": "EUR", "formatted": "€300,00"},
	}
	bs := parseBalanceSheet(res)
	if bs.Currency != "EUR" || bs.NetWorth != 5700 || bs.Assets != 6000 || bs.Liabilities != 300 {
		t.Fatalf("parsed = %+v", bs)
	}
}

func TestAmountFromAPIOK_FallbackToFormatted(t *testing.T) {
	// No cents field → fall back to parsing the formatted string.
	m := map[string]any{"balance": "$112.34"}
	got, ok := amountFromAPIOK(m, "balance", "balance_cents")
	if !ok {
		t.Fatal("expected ok=true via formatted fallback")
	}
	if got != 112.34 {
		t.Fatalf("got %v want 112.34", got)
	}
}

func TestAmountFromAPIOK_BothMissing(t *testing.T) {
	// Neither field present (nor parseable) → not ok, must not return a
	// stale/zero value as if it were truth.
	for _, m := range []map[string]any{
		{},
		{"balance": "", "balance_cents": nil},
		{"other": "noise"},
	} {
		if _, ok := amountFromAPIOK(m, "balance", "balance_cents"); ok {
			t.Fatalf("expected ok=false for %#v", m)
		}
	}
}

func TestAmountFromAPIOK_FallbackReadsLocalizedText(t *testing.T) {
	cases := []struct {
		text, currency string
		want           float64
	}{
		{"1.234,56 €", "EUR", 1234.56},
		{"R$ 12.345,00", "BRL", 12345},
		{"¥1,234", "JPY", 1234},
		{"ر.س12.50", "SAR", 12.5},
	}
	for _, c := range cases {
		got, ok := amountFromAPIOK(map[string]any{"balance": c.text, "currency": c.currency}, "balance", "balance_cents")
		if !ok || got != c.want {
			t.Errorf("%q: got %v ok=%v, want %v", c.text, got, ok, c.want)
		}
	}
}

func TestFormatMoneyValue(t *testing.T) {
	cases := []struct {
		value    float64
		currency string
		want     string
	}{
		{123.45, "USD", "123.45 USD"},
		{1.5, "EUR", "1.50 EUR"},
		{0, "GBP", "0.00 GBP"},
		// Each currency's own precision (Sure's default_precision).
		{0.00012345, "BTC", "0.00012345 BTC"},
		{0.5, "ETH", "0.50000000 ETH"},
		{1.2346, "KWD", "1.235 KWD"},
		{1234, "JPY", "1234 JPY"},
		{1.5, "MGA", "1.5 MGA"},
		// Empty / nil-stringified currency → no suffix (graceful for accounts
		// whose response lacks a currency field).
		{42.0, "", "42.00"},
		{42.0, "<nil>", "42.00"},
	}
	for _, c := range cases {
		got := formatMoneyValue(c.value, c.currency)
		if got != c.want {
			t.Fatalf("formatMoneyValue(%v, %q): got %q want %q", c.value, c.currency, got, c.want)
		}
	}
}

func monthlyRule(interval int) map[string]any {
	return map[string]any{"frequency": "monthly", "interval": float64(interval), "day_of_month": float64(1)}
}

func TestRecurringMonthlyOutflows_NormalisesCadencePerCurrency(t *testing.T) {
	series := []map[string]any{
		// Monthly subscription: counts as is.
		{"name": "Streaming", "amount_cents": float64(1299), "currency": "EUR", "bill_type": "subscription",
			"recurrence_rules": []any{monthlyRule(1)}},
		// Annual bill: 120 / 12.
		{"name": "Domain", "amount_cents": float64(12000), "currency": "EUR", "bill_type": "bill",
			"recurrence_rules": []any{map[string]any{"frequency": "yearly", "interval": float64(1), "day_of_month": float64(5), "month_of_year": float64(3)}}},
		// Semimonthly is two monthly rules: twice a month.
		{"name": "Cleaner", "amount_cents": float64(5000), "currency": "EUR", "bill_type": "bill",
			"recurrence_rules": []any{monthlyRule(1), monthlyRule(1)}},
		// No rules: Sure's legacy monthly series.
		{"name": "Gym", "amount_cents": float64(3000), "currency": "EUR", "bill_type": "subscription",
			"recurrence_rules": []any{}},
		// Every two weeks: 365.25 / 14 occurrences a year.
		{"name": "Lessons", "amount_cents": float64(1400), "currency": "USD", "bill_type": "bill",
			"recurrence_rules": []any{map[string]any{"frequency": "weekly", "interval": float64(2), "weekday": float64(5)}}},
		// JPY has no minor unit.
		{"name": "Phone", "amount_cents": float64(3000), "currency": "JPY", "bill_type": "bill",
			"recurrence_rules": []any{monthlyRule(1)}},
		// Not spending: income (negative amount) and transfers.
		{"name": "Salary", "amount_cents": float64(-250000), "currency": "EUR", "bill_type": "income",
			"recurrence_rules": []any{monthlyRule(1)}},
		{"name": "To savings", "amount_cents": float64(50000), "currency": "EUR", "bill_type": "transfer",
			"recurrence_rules": []any{monthlyRule(1)}},
		// One-off (Sure's FrequencyPreset ONCE): a payment, not a rate.
		{"name": "Visa fee", "amount_cents": float64(9900), "currency": "EUR", "bill_type": "bill",
			"end_mode": "after_count", "end_after_count": float64(1),
			"recurrence_rules": []any{map[string]any{"frequency": "yearly", "interval": float64(1), "day_of_month": float64(9), "month_of_year": float64(4)}}},
	}

	got := recurringMonthlyOutflows(series)

	wantEUR := 12.99 + 10 + 100 + 30
	if math.Abs(got["EUR"]-wantEUR) > 1e-9 {
		t.Fatalf("EUR = %v, want %v", got["EUR"], wantEUR)
	}
	wantUSD := 14 * (365.25 / 14) / 12
	if math.Abs(got["USD"]-wantUSD) > 1e-9 {
		t.Fatalf("USD = %v, want %v", got["USD"], wantUSD)
	}
	if got["JPY"] != 3000 {
		t.Fatalf("JPY = %v, want 3000", got["JPY"])
	}
	if len(got) != 3 {
		t.Fatalf("currencies = %v, want EUR, USD, JPY only", got)
	}
}

func TestRecurringMonthlyOutflows_InstallmentEndingAfterOneIsNotOneOff(t *testing.T) {
	// FrequencyPreset.once_schedule? excludes typed installments.
	series := []map[string]any{
		{"amount_cents": float64(10000), "currency": "EUR", "bill_type": "installment",
			"end_mode": "after_count", "end_after_count": float64(1),
			"recurrence_rules": []any{map[string]any{"frequency": "yearly", "interval": float64(1), "day_of_month": float64(9), "month_of_year": float64(4)}}},
	}
	if got := recurringMonthlyOutflows(series)["EUR"]; math.Abs(got-100.0/12) > 1e-9 {
		t.Fatalf("EUR = %v, want %v", got, 100.0/12)
	}
}

func TestBudgetPacing(t *testing.T) {
	budget := map[string]any{
		"id": "b1", "name": "March 2026", "currency": "EUR",
		"start_date": "2026-03-01", "end_date": "2026-03-31", "initialized": true,
		"budgeted_spending_cents":  float64(200000),
		"actual_spending_cents":    float64(50000),
		"available_to_spend_cents": float64(150000),
	}
	today := time.Date(2026, 3, 10, 15, 0, 0, 0, time.UTC)

	p := budgetPacing(budget, today)

	want := map[string]any{
		"id": "b1", "name": "March 2026", "currency": "EUR",
		"start_date": "2026-03-01", "end_date": "2026-03-31", "initialized": true,
		"budgeted_spending":  "2000.00 EUR",
		"actual_spending":    "500.00 EUR",
		"available_to_spend": "1500.00 EUR",
		"percent_spent":      25.0,
		"percent_elapsed":    math.Round(10.0/31*1000) / 10,
	}
	for k, v := range want {
		if p[k] != v {
			t.Fatalf("%s = %#v, want %#v (pacing %#v)", k, p[k], v, p)
		}
	}
}

func TestBudgetPacing_UninitializedHasNoBudgetedFigures(t *testing.T) {
	budget := map[string]any{
		"id": "b1", "currency": "EUR", "start_date": "2026-03-01", "end_date": "2026-03-31",
		"initialized": false, "budgeted_spending_cents": nil,
		"actual_spending_cents": float64(50000), "available_to_spend_cents": float64(-50000),
	}
	p := budgetPacing(budget, time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC))
	if p["budgeted_spending"] != nil || p["percent_spent"] != nil {
		t.Fatalf("uninitialized budget must not report budgeted figures: %#v", p)
	}
	if p["actual_spending"] != "500.00 EUR" {
		t.Fatalf("actual_spending = %#v", p["actual_spending"])
	}
}

// statusServer mirrors the endpoints `status` reads, for a family in UTC.
// GET /api/v1/cash_flow answers with cashFlowStatus and cashFlowBody, and
// its query lands in seen. It returns the server URL and how many requests
// each path got.
// statusAccounts are a EUR checking account and a EUR account excluded from
// reports, as GET /api/v1/accounts renders them.
const statusAccounts = `[
  {"id":"a1","name":"Checking","account_type":"depository","currency":"EUR","balance_cents":1000000,"cash_balance_cents":1000000,"classification":"asset","status":"active","exclude_from_reports":false},
  {"id":"a2","name":"Hidden","account_type":"depository","currency":"EUR","balance_cents":0,"cash_balance_cents":0,"classification":"asset","status":"active","exclude_from_reports":true}
]`

func statusServer(t *testing.T, cashFlowStatus int, cashFlowBody string, seen *url.Values) (string, func(path string) int) {
	t.Helper()
	return statusServerWithAccounts(t, statusAccounts, cashFlowStatus, cashFlowBody, seen)
}

// statusServerWithAccounts is statusServer with the given accounts list.
func statusServerWithAccounts(t *testing.T, accounts string, cashFlowStatus int, cashFlowBody string, seen *url.Values) (string, func(path string) int) {
	t.Helper()
	var mu sync.Mutex
	hits := map[string]int{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		hits[r.URL.Path]++
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/family_settings":
			fmt.Fprint(w, `{"timezone":"UTC"}`)
		case "/api/v1/balance_sheet":
			fmt.Fprint(w, `{"currency":"EUR","net_worth":{"amount":"10000.0","currency":"EUR"},"assets":{"amount":"10000.0","currency":"EUR"},"liabilities":{"amount":"0.0","currency":"EUR"}}`)
		case "/api/v1/accounts":
			fmt.Fprintf(w, `{"accounts":%s,"pagination":{"page":1,"total_pages":1}}`, accounts)
		case "/api/v1/cash_flow":
			mu.Lock()
			*seen = r.URL.Query()
			mu.Unlock()
			w.WriteHeader(cashFlowStatus)
			fmt.Fprint(w, cashFlowBody)
		case "/api/v1/recurring_transactions":
			fmt.Fprint(w, `{"recurring_transactions":[],"pagination":{"page":1,"total_pages":1}}`)
		case "/api/v1/budgets":
			fmt.Fprint(w, `{"budgets":[],"pagination":{"page":1,"total_pages":1}}`)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	apiClientFor(t, srv.URL)
	return srv.URL, func(path string) int {
		mu.Lock()
		defer mu.Unlock()
		return hits[path]
	}
}

// sankeyCashFlow is GET /api/v1/cash_flow?view=sankey as
// IncomeStatement::CashFlowGraph renders it, nodes and links left out.
func sankeyCashFlow(start, end, income, spending string) string {
	return fmt.Sprintf(`{"as_of":%[2]q,"time_zone":"UTC","currency":"EUR","period":{"start_date":%[1]q,"end_date":%[2]q},
	  "sankey":{"basis":"net_by_category","income":%[3]q,"spending":%[4]q,"invested":"0.0","net_savings":"0.0","nodes":[],"links":[]}}`,
		start, end, income, spending)
}

func TestStatus_MonthlyTotalsComeFromSureCashFlow(t *testing.T) {
	pinClock(t, testNow)
	end := today(testNow)
	start := lookbackStart(end, 1)
	var seen url.Values
	_, hits := statusServer(t, http.StatusOK,
		sankeyCashFlow(start.Format(time.DateOnly), end.Format(time.DateOnly), "3000.5", "4000.25"), &seen)

	cmd := newStatusCmd()
	cmd.SetArgs([]string{})
	out := captureStdout(t, func() { _ = cmd.Execute() })
	var env struct {
		Data struct {
			Monthly map[string]any   `json:"monthly"`
			Runway  map[string]any   `json:"runway"`
			Alerts  []map[string]any `json:"alerts"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("decode %s: %v", out, err)
	}

	want := url.Values{"view": {"sankey"}, "start_date": {start.Format(time.DateOnly)}, "end_date": {end.Format(time.DateOnly)}}
	if seen.Encode() != want.Encode() {
		t.Errorf("cash_flow query = %s, want %s", seen.Encode(), want.Encode())
	}
	for key, want := range map[string]string{"income": "3000.50 EUR", "expenses": "4000.25 EUR", "net": "-999.75 EUR"} {
		if got := env.Data.Monthly[key]; got != want {
			t.Errorf("monthly.%s = %v, want %s", key, got, want)
		}
	}
	if got := env.Data.Monthly["period"]; fmt.Sprint(got) != fmt.Sprint(map[string]any{"start_date": start.Format(time.DateOnly), "end_date": end.Format(time.DateOnly)}) {
		t.Errorf("monthly.period = %v, want Sure's period", got)
	}
	if got, want := env.Data.Runway["months"], 10000/4000.25; math.Abs(got.(float64)-want) > 1e-9 {
		t.Errorf("runway.months = %v, want %v", got, want)
	}
	if len(env.Data.Alerts) != 2 || env.Data.Alerts[0]["type"] != "high_burn" || env.Data.Alerts[1]["type"] != "low_runway" {
		t.Errorf("alerts = %v, want high_burn and low_runway", env.Data.Alerts)
	}
	if n := hits("/api/v1/transactions"); n != 0 {
		t.Errorf("read transactions %d times, want 0: Sure computes the totals", n)
	}
	if n := hits("/api/v1/accounts"); n != 1 {
		t.Errorf("read the accounts %d times, want once", n)
	}
}

func TestStatus_NoAlertsRenderAsEmptyList(t *testing.T) {
	var seen url.Values
	statusServer(t, http.StatusOK, sankeyCashFlow("2026-10-21", "2026-11-20", "5000", "1000"), &seen)
	cmd := newStatusCmd()
	cmd.SetArgs([]string{})
	out := captureStdout(t, func() { _ = cmd.Execute() })
	if !strings.Contains(out, `"alerts": []`) {
		t.Errorf("no alerts must render as [], got %s", out)
	}
}

func TestStatus_CashFlowFailures(t *testing.T) {
	for name, tc := range map[string]struct {
		status       int
		body         string
		wantCode     string
		wantHintPart string
	}{
		"older Sure without the endpoint": {http.StatusNotFound, `{"error":"not_found","message":"Not found"}`, "not_found", "Sure v0.7.5"},
		"totals missing":                  {http.StatusOK, `{"currency":"EUR","sankey":{}}`, "invalid_response", ""},
		"totals not decimals":             {http.StatusOK, `{"currency":"EUR","sankey":{"income":"lots","spending":"1"}}`, "invalid_response", ""},
	} {
		t.Run(name, func(t *testing.T) {
			var seen url.Values
			srvURL, _ := statusServer(t, tc.status, tc.body, &seen)
			child, stdout := startCLI(t, "--config", apiKeyConfig(t, t.TempDir(), srvURL), "status")
			if err := child.Wait(); err == nil {
				t.Fatalf("status exited 0; stdout %s", stdout)
			}
			var env struct {
				Error struct {
					Code    string         `json:"code"`
					Details map[string]any `json:"details"`
				} `json:"error"`
			}
			if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
				t.Fatalf("decode %s: %v", stdout, err)
			}
			if env.Error.Code != tc.wantCode {
				t.Errorf("code = %q, want %q (%s)", env.Error.Code, tc.wantCode, stdout)
			}
			if hint := fmt.Sprint(env.Error.Details["hint"]); !strings.Contains(hint, tc.wantHintPart) {
				t.Errorf("hint = %q, want it to mention %q", hint, tc.wantHintPart)
			}
		})
	}
}

func TestStatus_RunwayIsPartialWithCashInOtherCurrencies(t *testing.T) {
	// Sure converts spending in every currency to the family's; the API gives
	// no rate to convert the USD cash, so runway counts EUR cash only and must
	// not raise low_runway on that understated figure.
	accounts := `[
	  {"id":"a1","name":"Checking","account_type":"depository","currency":"EUR","balance_cents":250000,"cash_balance_cents":250000,"classification":"asset","status":"active","exclude_from_reports":false},
	  {"id":"a3","name":"US checking","account_type":"depository","currency":"USD","balance_cents":1000000,"cash_balance_cents":1000000,"classification":"asset","status":"active","exclude_from_reports":false}
	]`
	var seen url.Values
	statusServerWithAccounts(t, accounts, http.StatusOK, sankeyCashFlow("2026-10-21", "2026-11-20", "5000", "1000"), &seen)
	data := runStatus(t)
	runway, _ := data["runway"].(map[string]any)
	if runway["months"] != 2.5 || runway["partial"] != true || fmt.Sprint(runway["cash_currencies_left_out"]) != "[USD]" {
		t.Errorf("runway = %v, want 2.5 months from EUR cash, partial, USD left out", runway)
	}
	if a := fmt.Sprint(data["alerts"]); strings.Contains(a, "low_runway") {
		t.Errorf("alerts = %v, want no low_runway on a partial runway", a)
	}
}

func TestStatus_RunwayIsWholeWithCashOnlyInTheFamilyCurrency(t *testing.T) {
	var seen url.Values
	statusServer(t, http.StatusOK, sankeyCashFlow("2026-10-21", "2026-11-20", "5000", "1000"), &seen)
	runway, _ := runStatus(t)["runway"].(map[string]any)
	if runway["partial"] != false || fmt.Sprint(runway["cash_currencies_left_out"]) != "[]" {
		t.Errorf("runway = %v, want whole with nothing left out", runway)
	}
}

func runStatus(t *testing.T) map[string]any {
	t.Helper()
	cmd := newStatusCmd()
	cmd.SetArgs([]string{})
	out := captureStdout(t, func() { _ = cmd.Execute() })
	var env struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("decode %s: %v", out, err)
	}
	return env.Data
}

func TestDecimalValue_ReadsOnlyPlainDecimals(t *testing.T) {
	// Sure renders BigDecimal as plain text ("1234.5"); ParseFloat alone
	// would also take 1e5, NaN and Inf.
	for in, want := range map[any]float64{"1234.5": 1234.5, "-0.25": -0.25, float64(3): 3, " 7 ": 7} {
		if got, ok := decimalValue(in); !ok || got != want {
			t.Errorf("decimalValue(%#v) = %v, %v; want %v", in, got, ok, want)
		}
	}
	for _, in := range []any{"1e5", "NaN", "Inf", "", "abc", nil, true} {
		if got, ok := decimalValue(in); ok {
			t.Errorf("decimalValue(%#v) = %v, want not ok", in, got)
		}
	}
}
