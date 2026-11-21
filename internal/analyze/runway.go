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
// currency) from the expenses dated within [start, end] (whole days,
// inclusive) in that same currency. The expenses are the family's, from any
// account. Expenses in another currency cannot be netted without FX and are
// skipped; transactions with no currency are kept.
func ComputeRunway(balance float64, currency string, txs []models.Transaction, start, end time.Time) RunwaySummary {
	currency = strings.ToUpper(currency)
	first, last := day(start), day(end)
	windowDays := InclusiveDays(first, last)

	spent := 0.0
	for _, tx := range txs {
		if d := day(tx.Date); d.Before(first) || d.After(last) {
			continue
		}
		if tx.ReportClassification() != "expense" {
			continue
		}
		if tx.Currency != "" && !strings.EqualFold(tx.Currency, currency) {
			continue
		}
		spent += math.Abs(tx.Amount)
	}

	avgMonthly := spent / float64(windowDays) * 30.0
	// An overdrawn account has no runway, not a negative one.
	runway := 0.0
	if avgMonthly > 0 && balance > 0 {
		runway = balance / avgMonthly
	}

	return RunwaySummary{
		Balance:        balance,
		AvgMonthlyBurn: avgMonthly,
		RunwayMonths:   runway,
		Currency:       currency,
		WindowDays:     windowDays,
		Assumptions: []string{
			"burn is the whole family's expenses from every account, not only this account's",
			"expenses approximate Sure's income statement: no split parents, excluded rows, transfers or accounts excluded from reports; loan payments and investment contributions count as expenses (Sure's default family setting); burn extrapolated to 30-day month",
			"only expenses in the account currency count towards burn",
		},
	}
}
