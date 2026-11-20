package money

import "testing"

func TestMinorUnitConversion(t *testing.T) {
	cases := []struct {
		code string
		want float64
	}{
		{"EUR", 100},
		{"usd", 100},  // case-insensitive, like Money::Currency.new
		{"JPY", 1},    // zero-decimal
		{"KWD", 1000}, // three-decimal
		{"MGA", 5},    // non-decimal minor unit
		{"BTC", 1e8},
		{"", 100},      // missing currency → Sure's common default
		{"<nil>", 100}, // fmt.Sprint(nil) leaks from map[string]any plumbing
	}
	for _, c := range cases {
		if got := MinorUnitConversion(c.code); got != c.want {
			t.Errorf("MinorUnitConversion(%q) = %v, want %v", c.code, got, c.want)
		}
	}
}

func TestFromMinor(t *testing.T) {
	cases := []struct {
		minor float64
		code  string
		want  float64
	}{
		{123456, "EUR", 1234.56},
		{-2050, "USD", -20.50},
		{1500, "JPY", 1500},
		{12345, "KWD", 12.345},
		{11, "MGA", 2.2},
	}
	for _, c := range cases {
		if got := FromMinor(c.minor, c.code); got != c.want {
			t.Errorf("FromMinor(%v, %q) = %v, want %v", c.minor, c.code, got, c.want)
		}
	}
}

func TestPrecision(t *testing.T) {
	// Money::Currency#default_precision from Sure's config/currencies.yml.
	cases := []struct {
		code string
		want int
	}{
		{"EUR", 2}, {"usd", 2}, {"", 2},
		{"JPY", 0}, {"KWD", 3}, {"CLF", 4},
		{"BTC", 8}, {"ETH", 8}, // ETH counts 1e18 minor units but shows 8 decimals
		{"MGA", 1}, {"MRU", 1}, // five iraimbilanja to the ariary
	}
	for _, c := range cases {
		if got := Precision(c.code); got != c.want {
			t.Errorf("Precision(%q) = %d, want %d", c.code, got, c.want)
		}
	}
}

func TestMinorToDecimal(t *testing.T) {
	cases := []struct {
		minor, code, want string
	}{
		{"123456", "EUR", "1234.56"},
		{"-2050", "USD", "-20.5"},
		{"-5", "EUR", "-0.05"},
		{"0", "EUR", "0"},
		{"1500", "JPY", "1500"},
		{"12345", "KWD", "12.345"},
		{"11", "MGA", "2.2"},
		{"-7", "MRU", "-1.4"},
		{"12345678", "BTC", "0.12345678"},
		// Beyond int64 and float64: ETH counts 1e18 wei to the ether.
		{"12345678901234567891", "ETH", "12.345678901234567891"},
	}
	for _, c := range cases {
		got, err := MinorToDecimal(c.minor, c.code)
		if err != nil || got != c.want {
			t.Errorf("MinorToDecimal(%q, %q) = %q, %v; want %q", c.minor, c.code, got, err, c.want)
		}
	}
	for _, bad := range []string{"", "1.5", "1e3", "abc", "+-1"} {
		if _, err := MinorToDecimal(bad, "EUR"); err == nil {
			t.Errorf("MinorToDecimal(%q) succeeded, want error", bad)
		}
	}
}
