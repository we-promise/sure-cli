package analyze

import (
	"math"
	"strings"
	"time"

	"github.com/we-promise/sure-cli/internal/models"
)

type RunwaySummary struct {
	Balance        float64  `json:"balance"`
	AvgMonthlyBurn float64  `json:"avg_monthly_burn"`
	RunwayMonths   float64  `json:"runway_months"`
	Currency       string   `json:"currency"`
	WindowDays     int      `json:"window_days"`
	Assumptions    []string `json:"assumptions"`
}

// ComputeRunway estimates runway months for a balance (major units of
// currency) from recent expenses in that same currency. Expenses in another
// currency cannot be netted without FX and are skipped; transactions with no
// currency are kept.
func ComputeRunway(balance float64, currency string, txs []models.Transaction, windowDays int) RunwaySummary {
	currency = strings.ToUpper(currency)
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
		if tx.Currency != "" && !strings.EqualFold(tx.Currency, currency) {
			continue
		}
		spent += math.Abs(tx.Amount)
	}

	avgMonthly := 0.0
	if windowDays > 0 {
		avgDaily := spent / float64(windowDays)
		avgMonthly = avgDaily * 30.0
	}
	runway := 0.0
	if avgMonthly > 0 {
		runway = balance / avgMonthly
	}

	return RunwaySummary{
		Balance:        balance,
		AvgMonthlyBurn: avgMonthly,
		RunwayMonths:   runway,
		Currency:       currency,
		WindowDays:     windowDays,
		Assumptions: []string{
			"expenses counted as Sure reports count them (no split parents, excluded rows or transfers); burn extrapolated to 30-day month",
			"only expenses in the account currency count towards burn",
		},
	}
}
