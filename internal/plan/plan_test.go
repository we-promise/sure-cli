package plan

import (
	"testing"
	"time"

	"github.com/we-promise/sure-cli/internal/models"
)

func TestComputeRunway(t *testing.T) {
	now := time.Now().UTC()
	txs := []models.Transaction{
		{Classification: "expense", Amount: -10.00, Currency: "EUR", Date: now.AddDate(0, 0, -1)},
		{Classification: "expense", Amount: -20.00, Currency: "EUR", Date: now.AddDate(0, 0, -2)},
		{Classification: "income", Amount: 100.00, Currency: "EUR", Date: now.AddDate(0, 0, -2)},
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
		{Classification: "expense", Amount: -10, Currency: "USD", Date: now.AddDate(0, 0, -1)},
		// Different currency: cannot be netted against a USD balance without FX.
		{Classification: "expense", Amount: -500, Currency: "EUR", Date: now.AddDate(0, 0, -1)},
		// Currency unknown (older payloads): kept, as before.
		{Classification: "expense", Amount: -20, Date: now.AddDate(0, 0, -1)},
	}
	s := ComputeRunway(300, "usd", txs, 30)
	if s.Currency != "USD" {
		t.Fatalf("currency = %q, want USD", s.Currency)
	}
	if s.AvgMonthlyBurn != 30 {
		t.Fatalf("burn = %v, want 30 (EUR expense must be excluded)", s.AvgMonthlyBurn)
	}
}

func TestComputeRunway_SumsExpenseMagnitudes(t *testing.T) {
	now := time.Now().UTC()
	txs := []models.Transaction{
		{Classification: "expense", Amount: -10.5, Date: now.AddDate(0, 0, -1)},
		{Classification: "expense", Amount: -19.5, Date: now.AddDate(0, 0, -2)},
		{Classification: "income", Amount: 100, Date: now.AddDate(0, 0, -2)},
	}
	s := ComputeRunway(300, "EUR", txs, 30)
	if s.AvgMonthlyBurn != 30 || s.RunwayMonths != 10 {
		t.Fatalf("burn=%v runway=%v, want 30 and 10", s.AvgMonthlyBurn, s.RunwayMonths)
	}
}

func TestComputeMonthlyBudget_SumsExpenseMagnitudes(t *testing.T) {
	month := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	txs := []models.Transaction{
		{Classification: "expense", Amount: -12.25, Currency: "USD", Date: month.AddDate(0, 0, 3)},
		{Classification: "expense", Amount: -7.75, Currency: "USD", Date: month.AddDate(0, 0, 4)},
		{Classification: "income", Amount: 500, Currency: "USD", Date: month.AddDate(0, 0, 4)},
		{Classification: "expense", Amount: -99, Currency: "USD", Date: month.AddDate(0, 1, 0)}, // next month
	}
	b, err := ComputeMonthlyBudget(month, txs)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if b.Spent != 20 {
		t.Fatalf("spent = %v, want 20", b.Spent)
	}
	if b.Currency != "USD" {
		t.Fatalf("currency = %q, want the transactions' currency USD", b.Currency)
	}
}
