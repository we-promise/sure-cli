package insights

import (
	"testing"
	"time"
)

func TestDetectLeaks_SmallFrequent(t *testing.T) {
	txs := []Transaction{
		{ID: "1", Name: "Coffee", Classification: "expense", Amount: -2.50, Date: mustDate("2026-01-01")},
		{ID: "2", Name: "Coffee", Classification: "expense", Amount: -2.75, Date: mustDate("2026-01-02")},
		{ID: "3", Name: "Coffee", Classification: "expense", Amount: -2.25, Date: mustDate("2026-01-03")},
		{ID: "4", Name: "Rent", Classification: "expense", Amount: -900.00, Date: mustDate("2026-01-01")},
	}
	out := DetectLeaks(txs, 3, 5, 10)
	if len(out) != 1 {
		t.Fatalf("expected 1, got %d", len(out))
	}
	if out[0].Name != "Coffee" {
		t.Fatalf("expected Coffee")
	}
	if out[0].TotalAmount < 7.0 {
		t.Fatalf("expected total >= 7, got %v", out[0].TotalAmount)
	}
}

func mustDate(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}
