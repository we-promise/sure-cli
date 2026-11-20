// Package analyze holds client-side heuristics over Sure data that Sure does
// not compute itself.
package analyze

import (
	"math"
	"sort"
	"strings"
	"time"

	"github.com/we-promise/sure-cli/internal/models"
)

// LeakParams are the DetectLeaks thresholds. Amounts are major units of
// each candidate's own currency. A zero field takes its DefaultLeakParams
// value.
type LeakParams struct {
	MinCount int     `json:"min_count"`
	MinTotal float64 `json:"min_total"`
	MaxAvg   float64 `json:"max_avg"`
	// Currency, when set, keeps only expenses in that currency.
	Currency string `json:"currency,omitempty"`
}

// DefaultLeakParams are conservative on purpose, to keep noise out.
var DefaultLeakParams = LeakParams{MinCount: 3, MinTotal: 15, MaxAvg: 10}

func (p LeakParams) withDefaults() LeakParams {
	if p.MinCount <= 0 {
		p.MinCount = DefaultLeakParams.MinCount
	}
	if p.MinTotal <= 0 {
		p.MinTotal = DefaultLeakParams.MinTotal
	}
	if p.MaxAvg <= 0 {
		p.MaxAvg = DefaultLeakParams.MaxAvg
	}
	p.Currency = strings.ToUpper(p.Currency)
	return p
}

type LeakCandidate struct {
	// Name is the merchant's name when grouped by merchant, else the
	// transaction name.
	Name       string `json:"name"`
	MerchantID string `json:"merchant_id,omitempty"`
	// GroupedBy is "merchant" or "name".
	GroupedBy       string   `json:"grouped_by"`
	Currency        string   `json:"currency"`
	Count           int      `json:"count"`
	TotalAmount     float64  `json:"total_amount"` // positive
	AvgAmount       float64  `json:"avg_amount"`
	SpikeAmount     float64  `json:"spike_amount"`
	SampleTxIDs     []string `json:"sample_tx_ids"`
	Confidence      float64  `json:"confidence"`
	Reason          string   `json:"reason"`
	SuggestedAction string   `json:"suggested_action"`
}

// leakGroup is what expenses are grouped by: the merchant Sure matched (its
// id, else its name) or, without one, the transaction name; always within one
// currency, since amounts in different currencies cannot be added.
type leakGroup struct {
	by, key, currency string
}

func groupOf(tx models.Transaction) leakGroup {
	currency := strings.ToUpper(tx.Currency)
	switch {
	case tx.MerchantID != "":
		return leakGroup{"merchant", "id:" + tx.MerchantID, currency}
	case tx.MerchantName != "":
		return leakGroup{"merchant", "name:" + tx.MerchantName, currency}
	default:
		return leakGroup{"name", tx.Name, currency}
	}
}

// DetectLeaks finds "money leakage": small expenses that add up without
// being periodic subscriptions. It looks at expenses dated within
// [start, end] (whole days, inclusive) and keeps groups with at least
// MinCount occurrences, a total of at least MinTotal and an average of at
// most MaxAvg.
func DetectLeaks(txs []models.Transaction, start, end time.Time, p LeakParams) []LeakCandidate {
	p = p.withDefaults()
	first, last := day(start), day(end)

	groups := map[leakGroup][]models.Transaction{}
	for _, tx := range txs {
		if tx.Classification != "expense" {
			continue
		}
		if d := day(tx.Date); d.Before(first) || d.After(last) {
			continue
		}
		g := groupOf(tx)
		if p.Currency != "" && g.currency != p.Currency {
			continue
		}
		groups[g] = append(groups[g], tx)
	}

	var out []LeakCandidate
	for g, list := range groups {
		if len(list) < p.MinCount {
			continue
		}
		var total, spike float64
		ids := make([]string, 0, min(3, len(list)))
		for i, tx := range list {
			amt := math.Abs(tx.Amount)
			total += amt
			spike = math.Max(spike, amt)
			if i < 3 {
				ids = append(ids, tx.ID)
			}
		}
		avg := total / float64(len(list))
		if total < p.MinTotal || avg > p.MaxAvg {
			continue
		}

		conf := 0.6
		if len(list) >= 5 {
			conf += 0.15
		}
		if total >= 50 {
			conf += 0.1
		}

		action := "Track and set a budget"
		if total > 100 {
			action = "Significant leak; consider reducing frequency or finding alternatives"
		} else if len(list) >= 10 {
			action = "Very frequent small expense; batch or eliminate some occurrences"
		}

		c := LeakCandidate{
			Name:            list[0].Name,
			GroupedBy:       g.by,
			Currency:        g.currency,
			Count:           len(list),
			TotalAmount:     round2(total),
			AvgAmount:       round2(avg),
			SpikeAmount:     round2(spike),
			SampleTxIDs:     ids,
			Confidence:      conf,
			Reason:          "small_frequent_expenses",
			SuggestedAction: action,
		}
		if g.by == "merchant" {
			c.Name, c.MerchantID = list[0].MerchantName, list[0].MerchantID
		}
		out = append(out, c)
	}

	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Confidence != b.Confidence {
			return a.Confidence > b.Confidence
		}
		if a.TotalAmount != b.TotalAmount {
			return a.TotalAmount > b.TotalAmount
		}
		if a.Name != b.Name {
			return a.Name < b.Name
		}
		return a.Currency < b.Currency
	})
	return out
}

// day truncates t to its calendar date in UTC, as Sure's dates are parsed.
func day(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}
