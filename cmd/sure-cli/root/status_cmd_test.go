package root

import "testing"

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

func TestFormatMoneyValue(t *testing.T) {
	cases := []struct {
		value    float64
		currency string
		want     string
	}{
		{123.45, "USD", "123.45 USD"},
		{1.5, "EUR", "1.50 EUR"},
		{0, "GBP", "0.00 GBP"},
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
