package plan

import (
	"testing"
	"time"

	"github.com/we-promise/sure-cli/internal/models"
)

func TestComputeForecast(t *testing.T) {
	now := time.Now().UTC()
	
	// Create some historical transactions
	txs := []models.Transaction{
		// Regular expense (non-subscription)
		{ID: "1", Name: "Grocery Store", Classification: "expense", AmountText: "€50.00", Date: now.AddDate(0, 0, -5)},
		{ID: "2", Name: "Restaurant", Classification: "expense", AmountText: "€30.00", Date: now.AddDate(0, 0, -3)},
		{ID: "3", Name: "Gas Station", Classification: "expense", AmountText: "€40.00", Date: now.AddDate(0, 0, -1)},
		// Income (should be ignored)
		{ID: "4", Name: "Salary", Classification: "income", AmountText: "€2000.00", Date: now.AddDate(0, 0, -2)},
	}

	result := ComputeForecast(txs, 30, false)

	if result.Summary.Days != 30 {
		t.Errorf("expected Days=30, got %d", result.Summary.Days)
	}
	if result.Summary.ProjectedSpend <= 0 {
		t.Errorf("expected positive ProjectedSpend, got %f", result.Summary.ProjectedSpend)
	}
	if result.Summary.Currency != "EUR" {
		t.Errorf("expected Currency=EUR, got %s", result.Summary.Currency)
	}
	if len(result.Summary.Assumptions) == 0 {
		t.Error("expected assumptions to be populated")
	}
}

func TestComputeForecastWithDaily(t *testing.T) {
	now := time.Now().UTC()
	
	txs := []models.Transaction{
		{ID: "1", Name: "Coffee", Classification: "expense", AmountText: "€5.00", Date: now.AddDate(0, 0, -1)},
	}

	result := ComputeForecast(txs, 7, true)

	if result.Daily == nil {
		t.Fatal("expected daily forecast to be populated")
	}
	if len(result.Daily) != 7 {
		t.Errorf("expected 7 daily entries, got %d", len(result.Daily))
	}
	
	// Check cumulative spend increases
	for i := 1; i < len(result.Daily); i++ {
		if result.Daily[i].CumulativeSpend < result.Daily[i-1].CumulativeSpend {
			t.Error("cumulative spend should not decrease")
		}
	}
}
