// Package analyze holds client-side heuristics over Sure data that Sure does
// not compute itself.
package analyze

import (
	"errors"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/we-promise/sure-cli/internal/models"
	"github.com/we-promise/sure-cli/internal/money"
)

// LeakParams are the DetectLeaks thresholds, applied as given. Amounts are
// minor units (cents, yen, satoshis) of each group's currency, so one set of
// thresholds scales with every currency without FX rates.
type LeakParams struct {
	MinCount      int     `json:"min_count"`
	MinTotalMinor float64 `json:"min_total_minor"`
	MaxAvgMinor   float64 `json:"max_avg_minor"`
	// Currency, when set, keeps only expenses in that currency.
	Currency string `json:"currency,omitempty"`
}

// DefaultLeakParams are conservative on purpose, to keep noise out: 15.00
// total and 10.00 average in two-decimal currencies, ¥1500 and ¥1000 in yen.
var DefaultLeakParams = LeakParams{MinCount: 3, MinTotalMinor: 1500, MaxAvgMinor: 1000}

// Minor-unit totals that raise a candidate's confidence and change its
// suggested action (50.00 and 100.00 in two-decimal currencies).
const (
	notableTotalMinor     = 5000
	significantTotalMinor = 10000
)

// Validate rejects thresholds that would make every group pass or none.
func (p LeakParams) Validate() error {
	switch {
	case p.MinCount < 1:
		return errors.New("min_count must be at least 1")
	case p.MinTotalMinor < 0:
		return errors.New("min_total must not be negative")
	case p.MaxAvgMinor <= 0:
		return errors.New("max_avg must be positive (at least one minor unit)")
	}
	return nil
}

type LeakCandidate struct {
	// Name is the merchant's name when grouped by merchant, else the
	// transaction name.
	Name       string `json:"name"`
	MerchantID string `json:"merchant_id,omitempty"`
	// GroupedBy is "merchant" or "name".
	GroupedBy string `json:"grouped_by"`
	Currency  string `json:"currency"`
	Count     int    `json:"count"`
	// Amounts are positive major units, rounded to the currency's minor unit.
	TotalAmount     float64  `json:"total_amount"`
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
// MinCount occurrences, a total of at least MinTotalMinor and an average of
// at most MaxAvgMinor.
func DetectLeaks(txs []models.Transaction, start, end time.Time, p LeakParams) []LeakCandidate {
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
		if p.Currency != "" && !strings.EqualFold(g.currency, p.Currency) {
			continue
		}
		groups[g] = append(groups[g], tx)
	}

	var out []LeakCandidate
	for g, list := range groups {
		if len(list) < p.MinCount {
			continue
		}
		conv := money.MinorUnitConversion(g.currency)
		var totalMinor, spikeMinor float64
		ids := make([]string, 0, min(3, len(list)))
		for i, tx := range list {
			amt := math.Round(math.Abs(tx.Amount) * conv)
			totalMinor += amt
			spikeMinor = math.Max(spikeMinor, amt)
			if i < 3 {
				ids = append(ids, tx.ID)
			}
		}
		avgMinor := totalMinor / float64(len(list))
		if totalMinor < p.MinTotalMinor || avgMinor > p.MaxAvgMinor {
			continue
		}

		conf := 0.6
		if len(list) >= 5 {
			conf += 0.15
		}
		if totalMinor >= notableTotalMinor {
			conf += 0.1
		}

		action := "Track and set a budget"
		if totalMinor > significantTotalMinor {
			action = "Significant leak; consider reducing frequency or finding alternatives"
		} else if len(list) >= 10 {
			action = "Very frequent small expense; batch or eliminate some occurrences"
		}

		c := LeakCandidate{
			Name:            list[0].Name,
			GroupedBy:       g.by,
			Currency:        g.currency,
			Count:           len(list),
			TotalAmount:     totalMinor / conv,
			AvgAmount:       math.Round(avgMinor) / conv,
			SpikeAmount:     spikeMinor / conv,
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

// InclusiveDays counts the calendar days in [start, end], both ends included.
func InclusiveDays(start, end time.Time) int {
	return int(day(end).Sub(day(start)).Hours()/24) + 1
}

// day truncates t to its calendar date in UTC, as Sure's dates are parsed.
func day(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}
