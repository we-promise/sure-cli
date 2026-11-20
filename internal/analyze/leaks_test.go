package analyze

import (
	"math"
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
	out := DetectLeaks(txs, janStart, janEnd, LeakParams{MinCount: 3, MinTotalMinor: 500, MaxAvgMinor: 1000})
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
	out := DetectLeaks(txs, janStart, janEnd, LeakParams{MinCount: 3, MinTotalMinor: 500, MaxAvgMinor: 1000})
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
	out := DetectLeaks(txs, janStart, janEnd, LeakParams{MinCount: 3, MinTotalMinor: 500, MaxAvgMinor: 1000})
	if len(out) != 1 || out[0].Currency != "USD" || out[0].Count != 3 || out[0].TotalAmount != 9 {
		t.Fatalf("expected one USD group of 3 (EUR has only 2), got %+v", out)
	}

	onlyEUR := DetectLeaks(txs, janStart, janEnd, LeakParams{MinCount: 2, MinTotalMinor: 500, MaxAvgMinor: 1000, Currency: "eur"})
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
	out := DetectLeaks(txs, janStart, janEnd, LeakParams{MinCount: 3, MinTotalMinor: 500, MaxAvgMinor: 1000})
	if len(out) != 1 || out[0].Count != 3 || out[0].TotalAmount != 9 {
		t.Fatalf("got %+v, want the 3 January rows", out)
	}
}

func TestDetectLeaks_DefaultsScaleWithTheMinorUnit(t *testing.T) {
	var eur, jpy []models.Transaction
	for i, d := range []string{"2026-01-01", "2026-01-02", "2026-01-03"} {
		eur = append(eur, expense("e"+d, "Coffee", -5, d))
		jpy = append(jpy, models.Transaction{ID: "j" + string(rune('a'+i)), Name: "Konbini", Classification: "expense", Amount: -500, Currency: "JPY", Date: mustDate(d)})
	}
	// 15.00 EUR total, 5.00 average: exactly the 1500 / 1000 cent defaults.
	if out := DetectLeaks(eur, janStart, janEnd, DefaultLeakParams); len(out) != 1 {
		t.Fatalf("EUR at the defaults: got %+v", out)
	}
	// ¥1500 total, ¥500 average: yen have no cents, so the defaults are ¥1500 / ¥1000.
	out := DetectLeaks(jpy, janStart, janEnd, DefaultLeakParams)
	if len(out) != 1 || out[0].Currency != "JPY" || out[0].TotalAmount != 1500 || out[0].AvgAmount != 500 {
		t.Fatalf("JPY at the defaults: got %+v", out)
	}
	if DefaultLeakParams != (LeakParams{MinCount: 3, MinTotalMinor: 1500, MaxAvgMinor: 1000}) {
		t.Fatalf("defaults changed: %+v", DefaultLeakParams)
	}
}

func TestDetectLeaks_ZeroMinTotalIsApplied(t *testing.T) {
	txs := []models.Transaction{
		expense("1", "Gum", -0.01, "2026-01-01"),
		expense("2", "Gum", -0.01, "2026-01-02"),
	}
	out := DetectLeaks(txs, janStart, janEnd, LeakParams{MinCount: 2, MinTotalMinor: 0, MaxAvgMinor: 1})
	if len(out) != 1 || out[0].TotalAmount != 0.02 {
		t.Fatalf("a zero minimum total must keep tiny groups, got %+v", out)
	}
}

func TestDetectLeaks_RoundsToTheCurrencyMinorUnit(t *testing.T) {
	txs := []models.Transaction{
		{ID: "1", Name: "Fee", Classification: "expense", Amount: -0.0012, Currency: "BTC", Date: mustDate("2026-01-01")},
		{ID: "2", Name: "Fee", Classification: "expense", Amount: -0.0013, Currency: "BTC", Date: mustDate("2026-01-02")},
	}
	out := DetectLeaks(txs, janStart, janEnd, LeakParams{MinCount: 2, MaxAvgMinor: 1e6})
	if len(out) != 1 || out[0].TotalAmount != 0.0025 || out[0].SpikeAmount != 0.0013 {
		t.Fatalf("BTC amounts must keep 8 decimals, got %+v", out)
	}
}

// leakOf returns the single candidate for n expenses of amount each, or nil.
func leakOf(t *testing.T, n int, amount float64, p LeakParams) *LeakCandidate {
	t.Helper()
	var txs []models.Transaction
	for i := 0; i < n; i++ {
		txs = append(txs, expense(string(rune('a'+i)), "Snack", -amount, "2026-01-02"))
	}
	out := DetectLeaks(txs, janStart, janEnd, p)
	if len(out) > 1 {
		t.Fatalf("expected at most one candidate, got %+v", out)
	}
	if len(out) == 0 {
		return nil
	}
	return &out[0]
}

func TestDetectLeaks_ThresholdBoundaries(t *testing.T) {
	p := LeakParams{MinCount: 3, MinTotalMinor: 1500, MaxAvgMinor: 1000}
	if leakOf(t, 3, 5, p) == nil {
		t.Fatal("total == min_total must be kept")
	}
	if leakOf(t, 3, 4.99, p) != nil {
		t.Fatal("total below min_total must be dropped")
	}
	if leakOf(t, 2, 10, p) != nil {
		t.Fatal("count below min_count must be dropped")
	}
	if leakOf(t, 3, 10, p) == nil {
		t.Fatal("avg == max_avg must be kept")
	}
	if leakOf(t, 3, 10.01, p) != nil {
		t.Fatal("avg above max_avg must be dropped")
	}
}

func TestDetectLeaks_ConfidenceAndAction(t *testing.T) {
	p := LeakParams{MinCount: 1, MaxAvgMinor: 100000}
	cases := []struct {
		name   string
		n      int
		amount float64
		conf   float64
		action string
	}{
		{"base", 4, 1, 0.6, "Track and set a budget"},
		{"five occurrences", 5, 1, 0.75, "Track and set a budget"},
		{"total just under 50", 4, 12.49, 0.6, "Track and set a budget"},
		{"total of 50", 4, 12.5, 0.7, "Track and set a budget"},
		{"total of 100 is not significant yet", 4, 25, 0.7, "Track and set a budget"},
		{"total over 100", 4, 25.01, 0.7, "Significant leak; consider reducing frequency or finding alternatives"},
		{"nine occurrences", 9, 1, 0.75, "Track and set a budget"},
		{"ten occurrences", 10, 1, 0.75, "Very frequent small expense; batch or eliminate some occurrences"},
		{"significance wins over frequency", 10, 10.01, 0.85, "Significant leak; consider reducing frequency or finding alternatives"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := leakOf(t, c.n, c.amount, p)
			if got == nil {
				t.Fatal("expected a candidate")
			}
			if math.Abs(got.Confidence-c.conf) > 1e-9 || got.SuggestedAction != c.action {
				t.Fatalf("confidence %v action %q, want %v %q", got.Confidence, got.SuggestedAction, c.conf, c.action)
			}
		})
	}
}

func TestLeakParams_Validate(t *testing.T) {
	ok := LeakParams{MinCount: 1, MinTotalMinor: 0, MaxAvgMinor: 1}
	if err := ok.Validate(); err != nil {
		t.Fatalf("smallest valid params rejected: %v", err)
	}
	for name, p := range map[string]LeakParams{
		"min_count 0":        {MinCount: 0, MaxAvgMinor: 1},
		"negative min_total": {MinCount: 1, MinTotalMinor: -1, MaxAvgMinor: 1},
		"max_avg 0":          {MinCount: 1, MaxAvgMinor: 0},
	} {
		if p.Validate() == nil {
			t.Errorf("%s must be rejected", name)
		}
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
		out := DetectLeaks(txs, janStart, janEnd, LeakParams{MinCount: 3, MinTotalMinor: 500, MaxAvgMinor: 1000})
		if len(out) != 3 || out[0].Name != "Bus" || out[1].Name != "Coffee" || out[2].Name != "Tea" {
			t.Fatalf("ties must sort by name, got %+v", out)
		}
	}
}
