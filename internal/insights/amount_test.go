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
