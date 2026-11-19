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

	s, err := ComputeRunway("€300.00", txs, 30)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if s.AvgMonthlyBurn <= 0 {
		t.Fatalf("expected burn > 0")
	}
	if s.RunwayMonths <= 0 {
		t.Fatalf("expected runway > 0")
	}
}

func TestComputeRunway_SumsExpenseMagnitudes(t *testing.T) {
	now := time.Now().UTC()
	txs := []models.Transaction{
		{Classification: "expense", Amount: -10.5, Date: now.AddDate(0, 0, -1)},
		{Classification: "expense", Amount: -19.5, Date: now.AddDate(0, 0, -2)},
		{Classification: "income", Amount: 100, Date: now.AddDate(0, 0, -2)},
	}
	s, err := ComputeRunway("€300.00", txs, 30)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
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
