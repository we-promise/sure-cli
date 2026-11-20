package root

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/we-promise/sure-cli/internal/models"
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

func TestMonthlyTotals_UsesSignedAmountsAndNormalizesCurrency(t *testing.T) {
	txs := []models.Transaction{
		{Classification: "expense", Amount: -12.25, Currency: "EUR"},
		{Classification: "expense", Amount: -0.75},
		{Classification: "income", Amount: 2000.5, Currency: "eur"},
	}
	spend, income := monthlyTotals(txs, "EUR")
	if spend != 13 || income != 2000.5 {
		t.Fatalf("spend=%v income=%v, want 13 and 2000.5", spend, income)
	}
}

func TestMonthlyTotals_ClassifiesAsSureReports(t *testing.T) {
	// A loan payment is an expense whatever its sign, and so is an investment
	// contribution under Sure's default family setting.
	txs := []models.Transaction{
		{Classification: "income", Kind: "loan_payment", Amount: 300, Currency: "EUR"},
		{Classification: "income", Kind: "investment_contribution", Amount: 200, Currency: "EUR"},
		{Classification: "income", Kind: "standard", Amount: 1000, Currency: "EUR"},
	}
	spend, income := monthlyTotals(txs, "EUR")
	if spend != 500 || income != 1000 {
		t.Fatalf("spend=%v income=%v, want 500 and 1000", spend, income)
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

func TestMonthlyTotals_SkipsOtherCurrencies(t *testing.T) {
	txs := []models.Transaction{
		{Classification: "expense", Amount: -10, Currency: "EUR"},
		{Classification: "expense", Amount: -5000, Currency: "JPY"},
		{Classification: "income", Amount: 300, Currency: "USD"},
	}
	spend, income := monthlyTotals(txs, "EUR")
	if spend != 10 || income != 0 {
		t.Fatalf("spend=%v income=%v, want 10 and 0", spend, income)
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

// statusServer mirrors the endpoints `status` reads. Transactions are
// filtered by the requested window, as Sure filters them.
func statusServer(t *testing.T, transactions []map[string]any, seen *url.Values) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/balance_sheet":
			fmt.Fprint(w, `{"currency":"EUR","net_worth":{"amount":"10000.0","currency":"EUR"},"assets":{"amount":"10000.0","currency":"EUR"},"liabilities":{"amount":"0.0","currency":"EUR"}}`)
		case "/api/v1/accounts":
			fmt.Fprint(w, `{"accounts":[
			  {"id":"a1","name":"Checking","account_type":"depository","currency":"EUR","balance_cents":1000000,"cash_balance_cents":1000000,"classification":"asset","status":"active","exclude_from_reports":false},
			  {"id":"a2","name":"Hidden","account_type":"depository","currency":"EUR","balance_cents":0,"cash_balance_cents":0,"classification":"asset","status":"active","exclude_from_reports":true}
			],"pagination":{"page":1,"total_pages":1}}`)
		case "/api/v1/transactions":
			q := r.URL.Query()
			*seen = q
			var rows []map[string]any
			for _, tx := range transactions {
				d := tx["date"].(string)
				if d >= q.Get("start_date") && d <= q.Get("end_date") {
					rows = append(rows, tx)
				}
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"transactions": rows, "pagination": map[string]any{"page": 1, "total_pages": 1}})
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
}

func TestStatus_MonthlyWindowCountsAMonthlyBillOnce(t *testing.T) {
	now := time.Now().UTC()
	expense := func(id string, date time.Time, amount, account string) map[string]any {
		return map[string]any{"id": id, "date": date.Format(time.DateOnly), "amount": "€" + amount, "amount_decimal": amount,
			"currency": "EUR", "name": id, "classification": "expense", "kind": "standard", "account": map[string]any{"id": account, "name": account}}
	}
	var seen url.Values
	statusServer(t, []map[string]any{
		expense("rent-last-month", now.AddDate(0, -1, 0), "500", "a1"),
		expense("rent-this-month", now, "500", "a1"),
		expense("on-excluded-account", now, "300", "a2"),
	}, &seen)

	cmd := newStatusCmd()
	cmd.SetArgs([]string{})
	out := captureStdout(t, func() { _ = cmd.Execute() })
	var env struct {
		Data struct {
			Monthly map[string]any   `json:"monthly"`
			Alerts  []map[string]any `json:"alerts"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("decode %s: %v", out, err)
	}
	if got := env.Data.Monthly["expenses"]; got != "500.00 EUR" {
		t.Errorf("monthly expenses = %v, want 500.00 EUR (window %s..%s)", got, seen.Get("start_date"), seen.Get("end_date"))
	}
	if want := lookbackStart(now, 1).Format(time.DateOnly); seen.Get("start_date") != want {
		t.Errorf("start_date = %s, want %s", seen.Get("start_date"), want)
	}
	if env.Data.Alerts == nil || !strings.Contains(out, `"alerts": []`) {
		t.Errorf("no alerts must render as [], got %s", out)
	}
}
