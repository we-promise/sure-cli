package plan

import (
	"testing"
	"time"

	"github.com/we-promise/sure-cli/internal/models"
)

func TestComputeRunway(t *testing.T) {
	now := time.Now().UTC()
	txs := []models.Transaction{
		{Classification: "expense", AmountText: "€10.00", Currency: "EUR", Date: now.AddDate(0, 0, -1)},
		{Classification: "expense", AmountText: "€20.00", Currency: "EUR", Date: now.AddDate(0, 0, -2)},
		{Classification: "income", AmountText: "€100.00", Currency: "EUR", Date: now.AddDate(0, 0, -2)},
	}

	// Balance arrives numeric (from balance_cents), so a thousands separator
	// in the formatted string can no longer shrink it ("1.234,56 €" → 1.23456).
	s := ComputeRunway(1234.56, "EUR", txs, 30)
	if s.Balance != 1234.56 {
		t.Fatalf("balance = %v, want 1234.56", s.Balance)
	}
	if s.AvgMonthlyBurn != 30 {
		t.Fatalf("burn = %v, want 30", s.AvgMonthlyBurn)
	}
	if s.RunwayMonths != 1234.56/30 {
		t.Fatalf("runway = %v, want %v", s.RunwayMonths, 1234.56/30)
	}
	if s.Currency != "EUR" {
		t.Fatalf("currency = %q, want EUR", s.Currency)
	}
}

func TestComputeRunway_UsesAccountCurrency(t *testing.T) {
	now := time.Now().UTC()
	txs := []models.Transaction{
		{Classification: "expense", AmountText: "$10.00", Currency: "USD", Date: now.AddDate(0, 0, -1)},
		// Different currency: cannot be netted against a USD balance without FX.
		{Classification: "expense", AmountText: "€500.00", Currency: "EUR", Date: now.AddDate(0, 0, -1)},
		// Currency unknown (older payloads): kept, as before.
		{Classification: "expense", AmountText: "$20.00", Date: now.AddDate(0, 0, -1)},
	}
	s := ComputeRunway(300, "usd", txs, 30)
	if s.Currency != "USD" {
		t.Fatalf("currency = %q, want USD", s.Currency)
	}
	if s.AvgMonthlyBurn != 30 {
		t.Fatalf("burn = %v, want 30 (EUR expense must be excluded)", s.AvgMonthlyBurn)
	}
}
