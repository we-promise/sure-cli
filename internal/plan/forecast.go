package plan

import (
	"time"

	"github.com/we-promise/sure-cli/internal/insights"
	"github.com/we-promise/sure-cli/internal/models"
)

type ForecastSummary struct {
	Days              int      `json:"days"`
	RecurringExpenses float64  `json:"recurring_expenses"`
	AverageDailySpend float64  `json:"avg_daily_spend"`
	ProjectedSpend    float64  `json:"projected_spend"`
	Currency          string   `json:"currency"`
	Assumptions       []string `json:"assumptions"`
}

type DailyForecast struct {
	Date            string   `json:"date"`
	ExpectedSpend   float64  `json:"expected_spend"`
	CumulativeSpend float64  `json:"cumulative_spend"`
	RecurringItems  []string `json:"recurring_items,omitempty"`
}

type ForecastResult struct {
	Summary ForecastSummary `json:"summary"`
	Daily   []DailyForecast `json:"daily,omitempty"`
}

// ComputeForecast projects spending for the next N days based on:
// - detected recurring expenses (subscriptions)
// - average daily non-recurring spend
func ComputeForecast(txs []models.Transaction, days int, includeDaily bool) ForecastResult {
	if days <= 0 {
		days = 30
	}

	// Detect subscriptions for recurring
	subs := insights.DetectSubscriptions(txs)

	// Calculate average daily spend (non-subscription expenses)
	subNames := make(map[string]bool)
	for _, s := range subs {
		subNames[s.Name] = true
	}

	var nonRecurringTotal float64
	var expenseDays int
	daySet := make(map[string]bool)

	for _, tx := range txs {
		if tx.Classification != "expense" {
			continue
		}
		if subNames[tx.Name] {
			continue // skip recurring
		}
		amt, err := insights.ParseAmountEUR(tx.AmountText)
		if err != nil {
			continue
		}
		nonRecurringTotal += abs(amt)
		daySet[tx.Date.Format("2006-01-02")] = true
	}

	expenseDays = len(daySet)
	if expenseDays == 0 {
		expenseDays = 1
	}
	avgDailyNonRecurring := nonRecurringTotal / float64(expenseDays)

	// Calculate recurring expenses for forecast period
	var recurringTotal float64
	recurringByDay := make(map[string][]string)

	now := time.Now().UTC()
	for _, sub := range subs {
		// Estimate how many times this subscription will hit in the forecast period
		periodDays := sub.AvgPeriodDays
		if periodDays <= 0 {
			periodDays = 30
		}

		occurrences := float64(days) / periodDays
		recurringTotal += sub.AvgAmount * occurrences

		// For daily forecast, estimate when it will hit
		if includeDaily {
			nextHit := sub.LastDate
			for nextHit.Before(now) {
				nextHit = nextHit.AddDate(0, 0, int(periodDays))
			}
			for nextHit.Before(now.AddDate(0, 0, days)) {
				dateStr := nextHit.Format("2006-01-02")
				recurringByDay[dateStr] = append(recurringByDay[dateStr], sub.Name)
				nextHit = nextHit.AddDate(0, 0, int(periodDays))
			}
		}
	}

	projectedSpend := recurringTotal + (avgDailyNonRecurring * float64(days))

	result := ForecastResult{
		Summary: ForecastSummary{
			Days:              days,
			RecurringExpenses: round2(recurringTotal),
			AverageDailySpend: round2(avgDailyNonRecurring),
			ProjectedSpend:    round2(projectedSpend),
			Currency:          "EUR",
			Assumptions: []string{
				"recurring detected via subscription heuristics",
				"non-recurring extrapolated from historical average",
			},
		},
	}

	if includeDaily {
		var daily []DailyForecast
		var cumulative float64
		for i := 0; i < days; i++ {
			date := now.AddDate(0, 0, i)
			dateStr := date.Format("2006-01-02")

			daySpend := avgDailyNonRecurring
			var items []string
			if recItems, ok := recurringByDay[dateStr]; ok {
				items = recItems
				for _, name := range recItems {
					for _, sub := range subs {
						if sub.Name == name {
							daySpend += sub.AvgAmount
							break
						}
					}
				}
			}

			cumulative += daySpend
			daily = append(daily, DailyForecast{
				Date:            dateStr,
				ExpectedSpend:   round2(daySpend),
				CumulativeSpend: round2(cumulative),
				RecurringItems:  items,
			})
		}
		result.Daily = daily
	}

	return result
}

func round2(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
