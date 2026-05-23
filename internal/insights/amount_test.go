package insights

import "testing"

func TestParseAmountEUR(t *testing.T) {
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
		got, err := ParseAmountEUR(c.in)
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

func TestSignedAmount_UsesClassification(t *testing.T) {
	txIncome := Transaction{Classification: "income", AmountText: "-€2.00"}
	v, _ := SignedAmount(txIncome)
	if v != 2.0 {
		t.Fatalf("income signed amount: got %v", v)
	}
	txExpense := Transaction{Classification: "expense", AmountText: "€1.00"}
	v2, _ := SignedAmount(txExpense)
	if v2 != -1.0 {
		t.Fatalf("expense signed amount: got %v", v2)
	}
}
