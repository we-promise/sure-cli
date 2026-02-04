package plan

import (
	"time"

	"github.com/dgilperez/sure-cli/internal/insights"
	"github.com/dgilperez/sure-cli/internal/models"
)

type RunwaySummary struct {
	Balance        float64  `json:"balance"`
	AvgMonthlyBurn float64  `json:"avg_monthly_burn"`
	RunwayMonths   float64  `json:"runway_months"`
	Currency       string   `json:"currency"`
	WindowDays     int      `json:"window_days"`
	Assumptions    []string `json:"assumptions"`
}

// ComputeRunway estimates runway months based on recent spending.
func ComputeRunway(balanceText string, txs []models.Transaction, windowDays int) (RunwaySummary, error) {
	bal, err := insights.ParseAmountEUR(balanceText)
	cur := "EUR"
	if err != nil {
		return RunwaySummary{}, err
	}

	end := time.Now().UTC()
	start := end.AddDate(0, 0, -windowDays)

	spent := 0.0
	for _, tx := range txs {
		if tx.Date.Before(start) || tx.Date.After(end) {
			continue
		}
		if tx.Classification != "expense" {
			continue
		}
		amt, err := insights.ParseAmountEUR(tx.AmountText)
		if err != nil {
			continue
		}
		spent += amt
	}

	avgMonthly := 0.0
	if windowDays > 0 {
		avgDaily := spent / float64(windowDays)
		avgMonthly = avgDaily * 30.0
	}
	runway := 0.0
	if avgMonthly > 0 {
		runway = bal / avgMonthly
	}

	return RunwaySummary{
		Balance:        bal,
		AvgMonthlyBurn: avgMonthly,
		RunwayMonths:   runway,
		Currency:       cur,
		WindowDays:     windowDays,
		Assumptions:    []string{"expense sign normalized via classification; burn extrapolated to 30-day month"},
	}, nil
}
