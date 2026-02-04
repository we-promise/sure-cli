package insights

import (
	"testing"
	"time"
)

func TestDetectSubscriptions_MonthlyStable(t *testing.T) {
	txs := []Transaction{
		{ID: "1", Name: "Netflix", Classification: "expense", AmountText: "€9.99", Date: mustDate("2026-01-01")},
		{ID: "2", Name: "Netflix", Classification: "expense", AmountText: "€9.99", Date: mustDate("2026-02-01")},
		{ID: "3", Name: "Netflix", Classification: "expense", AmountText: "€9.99", Date: mustDate("2026-03-01")},
		{ID: "x", Name: "Coffee", Classification: "expense", AmountText: "€2.50", Date: mustDate("2026-03-02")},
	}
	out := DetectSubscriptions(txs)
	if len(out) != 1 {
		t.Fatalf("expected 1, got %d", len(out))
	}
	if out[0].Name != "Netflix" {
		t.Fatalf("expected Netflix")
	}
	if out[0].Count != 3 {
		t.Fatalf("expected count 3")
	}
	if out[0].AvgAmount != 9.99 {
		t.Fatalf("avg amount mismatch: %v", out[0].AvgAmount)
	}
}

func mustDate(s string) (t time.Time) {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}
