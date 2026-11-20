package models

import (
	"encoding/json"
	"testing"
)

func TestParseDisplayAmount(t *testing.T) {
	// Every shape Sure's Money#format renders (lib/money/formatting.rb):
	// the currency's own separators, the locale overrides (de/es/it, pl/nb/hu,
	// nl/pt-BR, fr with non-breaking spaces), Indian grouping, and symbols
	// that carry dots of their own.
	cases := []struct {
		display, currency, want string
	}{
		{"€1.00", "EUR", "1.00"},
		{"-€2.00", "EUR", "-2.00"},
		{"€1,50", "EUR", "1.50"},
		{" -€12,34 ", "EUR", "-12.34"},
		{"-€2,000.00", "EUR", "-2000.00"},
		{"€10,000.50", "EUR", "10000.50"},
		{"1.234,56 €", "EUR", "1234.56"},
		{"-1.234,56 €", "EUR", "-1234.56"},
		{"€1,234", "EUR", "1234"},
		{"$1,234.56", "USD", "1234.56"},
		{"US$1,234.56", "USD", "1234.56"},
		{"R$ 12.345,00", "BRL", "12345.00"},
		{"12,50 kr.", "DKK", "12.50"},
		{"1.234,56 kr.", "DKK", "1234.56"},
		{"1 234,56 zł", "PLN", "1234.56"},
		{"1 234,56 €", "EUR", "1234.56"},
		{"₹12,34,567.00", "INR", "1234567.00"},
		{"CHF 1'234.50", "CHF", "1234.50"},
		{"ر.س12.50", "SAR", "12.50"},
		{"Rs.12.50", "LKR", "12.50"},
		{"¥1,234", "JPY", "1234"},
		{"¥1.234", "JPY", "1234"},
		{"¥1.234.567", "JPY", "1234567"},
		{"KD1.234", "KWD", "1.234"},
		{"1,234 KD", "KWD", "1.234"},
		{"KD1,234.567", "KWD", "1234.567"},
		{"₿0.00012345", "BTC", "0.00012345"},
		{"Ar1.5", "MGA", "1.5"},
		{"£0.05", "GBP", "0.05"},
		{"$1,23", "", "1.23"},
		{"USD 12.34", "USD", "12.34"},
		{"+$3.14", "USD", "3.14"},
	}
	for _, c := range cases {
		got, err := ParseDisplayAmount(c.display, c.currency)
		if err != nil || got != c.want {
			t.Errorf("ParseDisplayAmount(%q, %q) = %q, %v; want %q", c.display, c.currency, got, err, c.want)
		}
		if err == nil && !IsDecimal(got) {
			t.Errorf("ParseDisplayAmount(%q) = %q, not in the decimal grammar", c.display, got)
		}
	}
}

func TestParseDisplayAmount_NoNumber(t *testing.T) {
	for _, in := range []string{"$", "USD", "  ", "", "-", "+", "ر.س", "Rs."} {
		if got, err := ParseDisplayAmount(in, "EUR"); err == nil {
			t.Errorf("ParseDisplayAmount(%q) = %q, want error", in, got)
		}
	}
}

func strPtr(s string) *string { return &s }

func numPtr(s string) *json.Number {
	n := json.Number(s)
	return &n
}

func TestResolveSignedAmount(t *testing.T) {
	cases := []struct {
		name      string
		fields    AmountFields
		wantValue float64
		wantText  string
	}{
		// amount_decimal is in the entry convention (positive = outflow); the
		// result flips it so expenses are negative and income positive.
		{"exact outflow keeps 4 decimals", AmountFields{Decimal: strPtr("12.3456"), SignedCents: numPtr("-1235"), Display: "€12.35", Currency: "EUR", Classification: "expense"}, -12.3456, "-12.3456"},
		{"exact inflow", AmountFields{Decimal: strPtr("-2000.0"), Display: "-€2,000.00", Classification: "income"}, 2000, "2000.0"},
		{"exact beats display rounding", AmountFields{Decimal: strPtr("0.005"), Display: "€0.01", Classification: "expense"}, -0.005, "-0.005"},
		{"exact zero has no sign", AmountFields{Decimal: strPtr("0.0"), Display: "€0.00", Classification: "expense"}, 0, "0.0"},
		{"exact value wins over classification", AmountFields{Decimal: strPtr("-5"), Display: "-€5.00", Classification: "expense"}, 5, "5"},
		// Servers from Sure #899 to #4422: signed_amount_cents is already
		// income positive, scaled by the currency's minor unit.
		{"cents expense", AmountFields{SignedCents: numPtr("-123456"), Display: "1.234,56 €", Currency: "EUR", Classification: "expense"}, -1234.56, "-1234.56"},
		{"cents income", AmountFields{SignedCents: numPtr("200000"), Display: "-€2,000.00", Currency: "EUR", Classification: "income"}, 2000, "2000"},
		{"cents JPY", AmountFields{SignedCents: numPtr("-1234"), Display: "¥1,234", Currency: "JPY", Classification: "expense"}, -1234, "-1234"},
		{"cents KWD", AmountFields{SignedCents: numPtr("-1234"), Display: "KD1.234", Currency: "KWD", Classification: "expense"}, -1.234, "-1.234"},
		{"cents beat unreadable display", AmountFields{SignedCents: numPtr("-1250"), Display: "ر.س12.50", Currency: "SAR", Classification: "expense"}, -12.5, "-12.5"},
		// Older servers: display text, sign from classification.
		{"legacy expense", AmountFields{Display: "€1,50", Currency: "EUR", Classification: "expense"}, -1.5, "-1.50"},
		{"legacy income", AmountFields{Display: "-€2.00", Currency: "EUR", Classification: "income"}, 2, "2.00"},
		{"legacy european grouping", AmountFields{Display: "1.234,56 €", Currency: "EUR", Classification: "expense"}, -1234.56, "-1234.56"},
		{"legacy JPY grouping", AmountFields{Display: "¥1,234", Currency: "JPY", Classification: "expense"}, -1234, "-1234"},
		{"legacy unknown classification flips entry sign", AmountFields{Display: "€3.00", Currency: "EUR"}, -3, "-3.00"},
		{"legacy zero has no sign", AmountFields{Display: "€0.00", Currency: "EUR", Classification: "expense"}, 0, "0.00"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			v, text, err := ResolveSignedAmount(c.fields)
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
	for name, f := range map[string]AmountFields{
		"non fixed-point amount_decimal": {Decimal: strPtr("1e3"), Display: "€1,000.00"},
		"empty amount_decimal":           {Decimal: strPtr(""), Display: "€1.00"},
		"fractional cents":               {SignedCents: numPtr("12.5"), Display: "€0.13"},
		"nothing readable":               {Display: ""},
	} {
		if _, _, err := ResolveSignedAmount(f); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}

func TestDecimalGrammar(t *testing.T) {
	for _, ok := range []string{"0", "1", "12.3456", "-0.5", "100.0000"} {
		if !IsDecimal(ok) {
			t.Errorf("IsDecimal(%q) = false", ok)
		}
	}
	// "5." is what BigDecimal() rejects; ".5" and the rest are not canonical.
	for _, bad := range []string{"", "-", ".5", "5.", "-.5", "1e3", "1,5", "+1", " 1", "abc", "1.2.3", "NaN"} {
		if IsDecimal(bad) {
			t.Errorf("IsDecimal(%q) = true", bad)
		}
	}
	for s, want := range map[string]bool{"1": true, "0.0001": true, "0": false, "0.000": false, "-1": false, "abc": false, "3.": false} {
		if got := IsPositiveDecimal(s); got != want {
			t.Errorf("IsPositiveDecimal(%q) = %v, want %v", s, got, want)
		}
	}
	for s, want := range map[string]int{"12": 0, "12.3400": 2, "-0.0001": 4, "1.0": 0} {
		if got := DecimalScale(s); got != want {
			t.Errorf("DecimalScale(%q) = %d, want %d", s, got, want)
		}
	}
	for s, want := range map[string]int{"-0012.5": 2, "0.5": 0, "123456789012345": 15} {
		if got := DecimalIntegerDigits(s); got != want {
			t.Errorf("DecimalIntegerDigits(%q) = %d, want %d", s, got, want)
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

func TestTransactionReportClassification(t *testing.T) {
	// IncomeStatement::ScopedTransactionsQuery#classification_sql: loan
	// payments are expenses whatever their sign; contributions too, under
	// Sure's default investment_contributions_as_spending.
	cases := []struct {
		tx   Transaction
		want string
	}{
		{Transaction{Kind: "standard", Classification: "expense"}, "expense"},
		{Transaction{Kind: "standard", Classification: "income"}, "income"},
		{Transaction{Classification: "income"}, "income"},
		{Transaction{Kind: "loan_payment", Classification: "income"}, "expense"},
		{Transaction{Kind: "investment_contribution", Classification: "income"}, "expense"},
	}
	for _, c := range cases {
		if got := c.tx.ReportClassification(); got != c.want {
			t.Errorf("%+v: ReportClassification() = %q, want %q", c.tx, got, c.want)
		}
	}
}
