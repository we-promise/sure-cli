package plan

import (
	"testing"
	"time"

	"github.com/dgilperez/sure-cli/internal/models"
)

func TestComputeRunway(t *testing.T) {
	now := time.Now().UTC()
	txs := []models.Transaction{
		{Classification: "expense", AmountText: "€10.00", Currency: "EUR", Date: now.AddDate(0, 0, -1)},
		{Classification: "expense", AmountText: "€20.00", Currency: "EUR", Date: now.AddDate(0, 0, -2)},
		{Classification: "income", AmountText: "€100.00", Currency: "EUR", Date: now.AddDate(0, 0, -2)},
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
