package root

import "testing"

func TestAmountFromAPIOK_CentsPreferredOverFormatted(t *testing.T) {
	// Cents wins even when a formatted string is also present, so a CLI
	// upgrade that starts sending *_cents doesn't silently round through the
	// string path.
	m := map[string]any{
		"balance":       "€999.99",  // would parse to 999.99
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
		{"float64", float64(12345), 123.45},                       // JSON numbers decode as float64
		{"int", int(12345), 123.45},                               // hand-built map literal in tests
		{"int64", int64(12345), 123.45},                           // some HTTP libs use int64
		{"string-numeric", "12345", 123.45},                       // some APIs emit cents as strings
		{"string-negative", "-2050", -20.50},                      // signed cents
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

func TestAmountFromAPI_ErrorWhenMissing(t *testing.T) {
	// The non-OK wrapper must surface an error rather than silently returning 0.
	if _, err := amountFromAPI(map[string]any{}, "balance", "balance_cents"); err == nil {
		t.Fatal("expected error when both fields are absent")
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
