package insights

import "testing"

func TestDetectFees_KeywordMatch(t *testing.T) {
	txs := []Transaction{
		{ID: "1", Name: "ATM Fee", Classification: "expense", AmountText: "€2.00", Date: mustDate("2026-01-01")},
		{ID: "2", Name: "ATM Fee", Classification: "expense", AmountText: "€2.00", Date: mustDate("2026-02-01")},
		{ID: "3", Name: "Salary", Classification: "income", AmountText: "-€100.00", Date: mustDate("2026-02-02")},
	}
	out := DetectFees(txs, []string{"fee"})
	if len(out) != 1 {
		t.Fatalf("expected 1, got %d", len(out))
	}
	if out[0].Name != "ATM Fee" {
		t.Fatalf("expected ATM Fee")
	}
	if out[0].TotalAmount != 4.0 {
		t.Fatalf("total mismatch: %v", out[0].TotalAmount)
	}
}
