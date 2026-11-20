package analyze

import (
	"testing"
	"time"

	"github.com/we-promise/sure-cli/internal/models"
)

// A 30-day window: January 2nd to 31st, both included.
var (
	runwayStart = mustDate("2026-01-02")
	runwayEnd   = mustDate("2026-01-31")
)

func TestComputeRunway(t *testing.T) {
	txs := []models.Transaction{
		{Classification: "expense", Amount: -10.00, Currency: "EUR", Date: mustDate("2026-01-30")},
		{Classification: "expense", Amount: -20.00, Currency: "EUR", Date: mustDate("2026-01-29")},
		{Classification: "income", Amount: 100.00, Currency: "EUR", Date: mustDate("2026-01-29")},
	}

	// Balance arrives numeric (from balance_cents), so a thousands separator
	// in the formatted string can no longer shrink it ("1.234,56 €" → 1.23456).
	s := ComputeRunway(1234.56, "EUR", txs, runwayStart, runwayEnd)
	if s.Balance != 1234.56 {
		t.Fatalf("balance = %v, want 1234.56", s.Balance)
	}
	if s.AvgMonthlyBurn != 30 {
		t.Fatalf("burn = %v, want 30", s.AvgMonthlyBurn)
	}
	if s.RunwayMonths != 1234.56/30 {
		t.Fatalf("runway = %v, want %v", s.RunwayMonths, 1234.56/30)
	}
	if s.Currency != "EUR" || s.WindowDays != 30 {
		t.Fatalf("currency = %q window = %d, want EUR and 30", s.Currency, s.WindowDays)
	}
}

func TestComputeRunway_CountsWholeDaysOfTheWindow(t *testing.T) {
	txs := []models.Transaction{
		{Classification: "expense", Amount: -500, Currency: "EUR", Date: mustDate("2026-01-01")}, // the day before
		{Classification: "expense", Amount: -15, Currency: "EUR", Date: mustDate("2026-01-02")},  // first day
		{Classification: "expense", Amount: -15, Currency: "EUR", Date: mustDate("2026-01-31")},  // last day
		{Classification: "expense", Amount: -500, Currency: "EUR", Date: mustDate("2026-02-01")}, // the day after
	}
	// Bounds with a time of day still mean whole calendar days.
	start := runwayStart.Add(18 * time.Hour)
	end := runwayEnd.Add(time.Hour)
	s := ComputeRunway(300, "EUR", txs, start, end)
	if s.WindowDays != 30 || s.AvgMonthlyBurn != 30 {
		t.Fatalf("window %d burn %v, want 30 days and 30 (both edge days counted, nothing outside)", s.WindowDays, s.AvgMonthlyBurn)
	}
}

func TestComputeRunway_UsesAccountCurrency(t *testing.T) {
	day := mustDate("2026-01-15")
	txs := []models.Transaction{
		{Classification: "expense", Amount: -10, Currency: "USD", Date: day},
		// Different currency: cannot be netted against a USD balance without FX.
		{Classification: "expense", Amount: -500, Currency: "EUR", Date: day},
		// Currency unknown (older payloads): kept, as before.
		{Classification: "expense", Amount: -20, Date: day},
	}
	s := ComputeRunway(300, "usd", txs, runwayStart, runwayEnd)
	if s.Currency != "USD" {
		t.Fatalf("currency = %q, want USD", s.Currency)
	}
	if s.AvgMonthlyBurn != 30 {
		t.Fatalf("burn = %v, want 30 (EUR expense must be excluded)", s.AvgMonthlyBurn)
	}
}

func TestComputeRunway_SumsExpenseMagnitudes(t *testing.T) {
	day := mustDate("2026-01-15")
	txs := []models.Transaction{
		{Classification: "expense", Amount: -10.5, Date: day},
		{Classification: "expense", Amount: -19.5, Date: day},
		{Classification: "income", Amount: 100, Date: day},
	}
	s := ComputeRunway(300, "EUR", txs, runwayStart, runwayEnd)
	if s.AvgMonthlyBurn != 30 || s.RunwayMonths != 10 {
		t.Fatalf("burn=%v runway=%v, want 30 and 10", s.AvgMonthlyBurn, s.RunwayMonths)
	}
}

func TestInclusiveDays(t *testing.T) {
	if got := InclusiveDays(runwayStart, runwayStart.Add(23*time.Hour)); got != 1 {
		t.Fatalf("same day = %d, want 1", got)
	}
	if got := InclusiveDays(mustDate("2026-03-01"), mustDate("2026-03-31")); got != 31 {
		t.Fatalf("March = %d, want 31", got)
	}
}
