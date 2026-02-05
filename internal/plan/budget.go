package plan

import (
	"time"

	"github.com/we-promise/sure-cli/internal/insights"
	"github.com/we-promise/sure-cli/internal/models"
)

type BudgetSummary struct {
	Month       string   `json:"month"`
	DaysElapsed int      `json:"days_elapsed"`
	DaysInMonth int      `json:"days_in_month"`
	Spent       float64  `json:"spent"`
	AvgPerDay   float64  `json:"avg_per_day"`
	Projected   float64  `json:"projected"`
	Currency    string   `json:"currency"`
	Assumptions []string `json:"assumptions"`
}

// ComputeMonthlyBudget is a lightweight client-side budget pacing view.
// It sums expenses in the month and projects based on average daily spend so far.
func ComputeMonthlyBudget(month time.Time, txs []models.Transaction) (BudgetSummary, error) {
	start := time.Date(month.Year(), month.Month(), 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)

	spent := 0.0
	currency := ""
	for _, tx := range txs {
		if tx.Date.Before(start) || !tx.Date.Before(end) {
			continue
		}
		if tx.Classification != "expense" {
			continue
		}
		amt, err := insights.ParseAmountEUR(tx.AmountText)
		if err != nil {
			continue
		}
		currency = "EUR"
		spent += amt
	}

	now := time.Now().UTC()
	daysElapsed := int(now.Sub(start).Hours()/24) + 1
	if now.Before(start) {
		daysElapsed = 0
	}
	if now.After(end) {
		daysElapsed = int(end.Sub(start).Hours() / 24)
	}
	daysInMonth := int(end.Sub(start).Hours() / 24)

	avg := 0.0
	if daysElapsed > 0 {
		avg = spent / float64(daysElapsed)
	}
	projected := avg * float64(daysInMonth)

	return BudgetSummary{
		Month:       start.Format("2006-01"),
		DaysElapsed: daysElapsed,
		DaysInMonth: daysInMonth,
		Spent:       spent,
		AvgPerDay:   avg,
		Projected:   projected,
		Currency:    currency,
		Assumptions: []string{"expense sign normalized via classification; uses month-to-date average"},
	}, nil
}
