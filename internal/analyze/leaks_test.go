package analyze

import (
	"testing"
	"time"

	"github.com/we-promise/sure-cli/internal/models"
)

func mustDate(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}

func expense(id, name string, amount float64, date string) models.Transaction {
	return models.Transaction{ID: id, Name: name, Classification: "expense", Amount: amount, Currency: "EUR", Date: mustDate(date)}
}

var (
	janStart = mustDate("2026-01-01")
	janEnd   = mustDate("2026-01-31")
)

func TestDetectLeaks_SmallFrequent(t *testing.T) {
	txs := []models.Transaction{
		expense("1", "Coffee", -2.50, "2026-01-01"),
		expense("2", "Coffee", -2.75, "2026-01-02"),
		expense("3", "Coffee", -2.25, "2026-01-03"),
		expense("4", "Rent", -900.00, "2026-01-01"),
	}
	out := DetectLeaks(txs, janStart, janEnd, LeakParams{MinCount: 3, MinTotal: 5, MaxAvg: 10})
	if len(out) != 1 {
		t.Fatalf("expected 1, got %d: %+v", len(out), out)
	}
	c := out[0]
	if c.Name != "Coffee" || c.GroupedBy != "name" || c.Currency != "EUR" || c.Count != 3 || c.TotalAmount != 7.5 || c.AvgAmount != 2.5 {
		t.Fatalf("candidate = %+v", c)
	}
}

func TestDetectLeaks_GroupsByMerchantBeforeName(t *testing.T) {
	// Bank descriptors vary per charge; the merchant Sure matched does not.
	txs := []models.Transaction{
		{ID: "1", Name: "STARBUCKS 0412 MADRID", MerchantID: "m-sb", MerchantName: "Starbucks", Classification: "expense", Amount: -3, Currency: "EUR", Date: mustDate("2026-01-02")},
		{ID: "2", Name: "STARBUCKS 0977 MADRID", MerchantID: "m-sb", MerchantName: "Starbucks", Classification: "expense", Amount: -4, Currency: "EUR", Date: mustDate("2026-01-09")},
		{ID: "3", Name: "Starbucks Coffee", MerchantID: "m-sb", MerchantName: "Starbucks", Classification: "expense", Amount: -5, Currency: "EUR", Date: mustDate("2026-01-16")},
		// Same descriptor as a merchant row but no merchant: grouped by name, apart.
		{ID: "4", Name: "Starbucks Coffee", Classification: "expense", Amount: -5, Currency: "EUR", Date: mustDate("2026-01-20")},
	}
	out := DetectLeaks(txs, janStart, janEnd, LeakParams{MinCount: 3, MinTotal: 5, MaxAvg: 10})
	if len(out) != 1 {
		t.Fatalf("expected the merchant group only, got %+v", out)
	}
	c := out[0]
	if c.Name != "Starbucks" || c.MerchantID != "m-sb" || c.GroupedBy != "merchant" || c.Count != 3 || c.TotalAmount != 12 {
		t.Fatalf("candidate = %+v", c)
	}
}

func TestDetectLeaks_NeverAddsAcrossCurrencies(t *testing.T) {
	txs := []models.Transaction{
		{ID: "1", Name: "Coffee", Classification: "expense", Amount: -3, Currency: "EUR", Date: mustDate("2026-01-02")},
		{ID: "2", Name: "Coffee", Classification: "expense", Amount: -3, Currency: "EUR", Date: mustDate("2026-01-03")},
		{ID: "3", Name: "Coffee", Classification: "expense", Amount: -3, Currency: "USD", Date: mustDate("2026-01-04")},
		{ID: "4", Name: "Coffee", Classification: "expense", Amount: -3, Currency: "usd", Date: mustDate("2026-01-05")},
		{ID: "5", Name: "Coffee", Classification: "expense", Amount: -3, Currency: "USD", Date: mustDate("2026-01-06")},
	}
	out := DetectLeaks(txs, janStart, janEnd, LeakParams{MinCount: 3, MinTotal: 5, MaxAvg: 10})
	if len(out) != 1 || out[0].Currency != "USD" || out[0].Count != 3 || out[0].TotalAmount != 9 {
		t.Fatalf("expected one USD group of 3 (EUR has only 2), got %+v", out)
	}

	onlyEUR := DetectLeaks(txs, janStart, janEnd, LeakParams{MinCount: 2, MinTotal: 5, MaxAvg: 10, Currency: "eur"})
	if len(onlyEUR) != 1 || onlyEUR[0].Currency != "EUR" || onlyEUR[0].Count != 2 {
		t.Fatalf("currency filter: got %+v", onlyEUR)
	}
}

func TestDetectLeaks_OnlyCountsTheWindow(t *testing.T) {
	txs := []models.Transaction{
		expense("0", "Coffee", -3, "2025-12-31"), // before
		expense("1", "Coffee", -3, "2026-01-01"), // first day, inclusive
		expense("2", "Coffee", -3, "2026-01-15"),
		expense("3", "Coffee", -3, "2026-01-31"), // last day, inclusive
		expense("4", "Coffee", -3, "2026-02-01"), // after
	}
	out := DetectLeaks(txs, janStart, janEnd, LeakParams{MinCount: 3, MinTotal: 5, MaxAvg: 10})
	if len(out) != 1 || out[0].Count != 3 || out[0].TotalAmount != 9 {
		t.Fatalf("got %+v, want the 3 January rows", out)
	}
}

func TestDetectLeaks_ZeroParamsUseDefaults(t *testing.T) {
	txs := []models.Transaction{
		expense("1", "Coffee", -5, "2026-01-01"),
		expense("2", "Coffee", -5, "2026-01-02"),
		expense("3", "Coffee", -5, "2026-01-03"),
	}
	// 15 total, 5 average: meets DefaultLeakParams (3 / 15 / 10).
	if out := DetectLeaks(txs, janStart, janEnd, LeakParams{}); len(out) != 1 {
		t.Fatalf("got %+v", out)
	}
	if DefaultLeakParams != (LeakParams{MinCount: 3, MinTotal: 15, MaxAvg: 10}) {
		t.Fatalf("defaults changed: %+v", DefaultLeakParams)
	}
}

func TestDetectLeaks_StableOrder(t *testing.T) {
	var txs []models.Transaction
	for _, name := range []string{"Tea", "Bus", "Coffee"} {
		for i, day := range []string{"2026-01-01", "2026-01-02", "2026-01-03"} {
			txs = append(txs, expense(name+string(rune('a'+i)), name, -6, day))
		}
	}
	for i := 0; i < 20; i++ {
		out := DetectLeaks(txs, janStart, janEnd, LeakParams{MinCount: 3, MinTotal: 5, MaxAvg: 10})
		if len(out) != 3 || out[0].Name != "Bus" || out[1].Name != "Coffee" || out[2].Name != "Tea" {
			t.Fatalf("ties must sort by name, got %+v", out)
		}
	}
}
