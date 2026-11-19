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
