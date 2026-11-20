package models

import "testing"

func TestParseAmount_EUR(t *testing.T) {
	cases := []struct {
		in  string
		out float64
	}{
		{"€1.00", 1.0},
		{"-€2.00", -2.0},
		{"€1,50", 1.5},
		{" -€12,34 ", -12.34},
		{"-€2,000.00", -2000.0},
		{"€10,000.50", 10000.5},
	}
	for _, c := range cases {
		got, err := ParseAmount(c.in)
		if err != nil {
			t.Fatalf("err for %q: %v", c.in, err)
		}
		if got != c.out {
			t.Fatalf("%q: got %v want %v", c.in, got, c.out)
		}
	}
}

func TestParseAmount_NonEURCurrencies(t *testing.T) {
	cases := []struct {
		in  string
		out float64
	}{
		// Symbols other than €
		{"$12.34", 12.34},
		{"-$2.00", -2.0},
		{"£1,000.50", 1000.5},
		{"¥9876.54", 9876.54},
		{"₹500", 500.0},
		// ISO code prefixes (letters get stripped by the unicode.IsDigit filter)
		{"USD 12.34", 12.34},
		{"-GBP 2.50", -2.5},
		// Surrounding whitespace
		{"  $42.00  ", 42.0},
		// Explicit plus sign
		{"+$3.14", 3.14},
		// Decimal comma with a non-EUR symbol
		{"$1,23", 1.23},
	}
	for _, c := range cases {
		got, err := ParseAmount(c.in)
		if err != nil {
			t.Fatalf("err for %q: %v", c.in, err)
		}
		if got != c.out {
			t.Fatalf("%q: got %v want %v", c.in, got, c.out)
		}
	}
}

func TestParseAmount_EmptyAfterStrip(t *testing.T) {
	// Inputs that strip down to nothing (or just a sign) must error,
	// not silently return 0.
	for _, in := range []string{"$", "USD", "  ", "", "-", "+"} {
		if _, err := ParseAmount(in); err == nil {
			t.Fatalf("expected error for %q", in)
		}
	}
}

func strPtr(s string) *string { return &s }

func TestResolveSignedAmount(t *testing.T) {
	cases := []struct {
		name           string
		decimal        *string
		display        string
		classification string
		wantValue      float64
		wantText       string
	}{
		// amount_decimal is in the entry convention (positive = outflow); the
		// result flips it so expenses are negative and income positive.
		{"exact outflow keeps 4 decimals", strPtr("12.3456"), "€12.35", "expense", -12.3456, "-12.3456"},
		{"exact inflow", strPtr("-2000.0"), "-€2,000.00", "income", 2000, "2000.0"},
		{"exact beats display rounding", strPtr("0.005"), "€0.01", "expense", -0.005, "-0.005"},
		{"exact zero has no sign", strPtr("0.0"), "€0.00", "expense", 0, "0.0"},
		{"exact value wins over classification", strPtr("-5"), "-€5.00", "expense", 5, "5"},
		// Servers before amount_decimal: display text, sign from classification.
		{"legacy expense", nil, "€1,50", "expense", -1.5, "-1.5"},
		{"legacy income", nil, "-€2.00", "income", 2, "2"},
		{"legacy unknown classification flips entry sign", nil, "€3.00", "", -3, "-3"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			v, text, err := ResolveSignedAmount(c.decimal, c.display, c.classification)
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if v != c.wantValue || text != c.wantText {
				t.Fatalf("got (%v, %q), want (%v, %q)", v, text, c.wantValue, c.wantText)
			}
		})
	}
}

func TestResolveSignedAmount_Errors(t *testing.T) {
	if _, _, err := ResolveSignedAmount(strPtr("1e3"), "€1,000.00", "expense"); err == nil {
		t.Fatal("expected error for non fixed-point amount_decimal")
	}
	if _, _, err := ResolveSignedAmount(strPtr(""), "€1.00", "expense"); err == nil {
		t.Fatal("expected error for empty amount_decimal")
	}
	if _, _, err := ResolveSignedAmount(nil, "", "expense"); err == nil {
		t.Fatal("expected error when neither amount_decimal nor amount parse")
	}
}

func TestIsDecimal(t *testing.T) {
	for _, ok := range []string{"0", "1", "12.3456", "-0.5", "100.0000"} {
		if !IsDecimal(ok) {
			t.Fatalf("IsDecimal(%q) = false", ok)
		}
	}
	for _, bad := range []string{"", "-", ".5", "5.", "1e3", "1,5", "+1", " 1", "abc", "1.2.3", "NaN"} {
		if IsDecimal(bad) {
			t.Fatalf("IsDecimal(%q) = true", bad)
		}
	}
}

func TestTransactionReportable(t *testing.T) {
	cases := []struct {
		name string
		tx   Transaction
		want bool
	}{
		{"plain expense", Transaction{Kind: "standard"}, true},
		{"server without kind", Transaction{}, true},
		{"loan payment counts", Transaction{Kind: "loan_payment"}, true},
		{"investment contribution counts", Transaction{Kind: "investment_contribution"}, true},
		{"excluded", Transaction{Excluded: true}, false},
		{"split parent", Transaction{SplitParent: true}, false},
		{"funds movement", Transaction{Kind: "funds_movement"}, false},
		{"credit card payment", Transaction{Kind: "cc_payment"}, false},
		{"one time", Transaction{Kind: "one_time"}, false},
	}
	for _, c := range cases {
		if got := c.tx.Reportable(); got != c.want {
			t.Fatalf("%s: Reportable() = %v, want %v", c.name, got, c.want)
		}
	}
}
