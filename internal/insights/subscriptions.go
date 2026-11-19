package insights

import (
	"fmt"
	"math"
	"sort"
	"time"
)

type SubscriptionCandidate struct {
	Name            string    `json:"name"`
	Count           int       `json:"count"`
	AvgAmount       float64   `json:"avg_amount"`
	AvgPeriodDays   float64   `json:"avg_period_days"`
	StdDevDays      float64   `json:"stddev_days"`
	LastDate        time.Time `json:"last_date"`
	SampleTxIDs     []string  `json:"sample_tx_ids"`
	Classification  string    `json:"classification"` // usually expense
	Confidence      float64   `json:"confidence"`
	Reason          string    `json:"reason"`
	SuggestedAction string    `json:"suggested_action"`
}

// DetectSubscriptions finds recurring transactions by same name with roughly regular spacing and stable amounts.
// Heuristic: at least 3 occurrences, avg period between 20-40 days OR 6-9 days (weekly), stddev <= 3 days.
func DetectSubscriptions(txs []Transaction) []SubscriptionCandidate {
	byName := map[string][]Transaction{}
	for _, tx := range txs {
		if tx.Classification != "expense" {
			continue
		}
		byName[tx.Name] = append(byName[tx.Name], tx)
	}

	var out []SubscriptionCandidate
	for name, list := range byName {
		if len(list) < 3 {
			continue
		}
		sort.Slice(list, func(i, j int) bool { return list[i].Date.Before(list[j].Date) })

		// compute deltas
		days := make([]float64, 0, len(list)-1)
		for i := 1; i < len(list); i++ {
			d := list[i].Date.Sub(list[i-1].Date).Hours() / 24.0
			days = append(days, d)
		}
		avg, std := meanStd(days)

		monthly := avg >= 20 && avg <= 40
		weekly := avg >= 6 && avg <= 9
		if !(monthly || weekly) {
			continue
		}
		if std > 3.0 {
			continue
		}

		// amounts
		amounts := make([]float64, 0, len(list))
		ids := make([]string, 0, min(3, len(list)))
		for i, tx := range list {
			amounts = append(amounts, math.Abs(tx.Amount))
			if i >= len(list)-3 {
				ids = append(ids, tx.ID)
			}
		}
		avgAmt, stdAmt := meanStd(amounts)
		// stable amount: std dev < 10% of mean (or < 1€)
		stable := (avgAmt > 0 && stdAmt/avgAmt < 0.1) || stdAmt < 1.0
		if !stable {
			continue
		}

		conf := 0.7
		if monthly {
			conf += 0.1
		}
		if std < 1.0 {
			conf += 0.1
		}
		if stable {
			conf += 0.1
		}
		if conf > 1.0 {
			conf = 1.0
		}

		reason := "recurring_expense"
		if monthly {
			reason = "monthly_recurring"
		} else if weekly {
			reason = "weekly_recurring"
		}

		action := "Review if still needed"
		if avgAmt > 20 {
			action = "Review if still needed; consider canceling to save ~" + formatAmount(avgAmt*12) + "/year"
		}

		out = append(out, SubscriptionCandidate{
			Name:            name,
			Count:           len(list),
			AvgAmount:       round2(avgAmt),
			AvgPeriodDays:   round2(avg),
			StdDevDays:      round2(std),
			LastDate:        list[len(list)-1].Date,
			SampleTxIDs:     ids,
			Classification:  "expense",
			Confidence:      conf,
			Reason:          reason,
			SuggestedAction: action,
		})
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Confidence == out[j].Confidence {
			return out[i].AvgAmount > out[j].AvgAmount
		}
		return out[i].Confidence > out[j].Confidence
	})
	return out
}

func meanStd(xs []float64) (mean float64, std float64) {
	if len(xs) == 0 {
		return 0, 0
	}
	for _, x := range xs {
		mean += x
	}
	mean /= float64(len(xs))
	var v float64
	for _, x := range xs {
		d := x - mean
		v += d * d
	}
	v /= float64(len(xs))
	std = math.Sqrt(v)
	return mean, std
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func formatAmount(v float64) string {
	return fmt.Sprintf("€%.0f", v)
}
