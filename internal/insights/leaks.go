package insights

import (
	"math"
	"sort"
)

type LeakCandidate struct {
	Name        string   `json:"name"`
	Count       int      `json:"count"`
	TotalAmount float64  `json:"total_amount"` // positive
	AvgAmount   float64  `json:"avg_amount"`
	SpikeAmount float64  `json:"spike_amount"`
	SampleTxIDs []string `json:"sample_tx_ids"`
	Confidence  float64  `json:"confidence"`
	Reason      string   `json:"reason"`
}

// DetectLeaks finds “money leakage” patterns: small recurring-ish expenses that add up,
// without necessarily being perfectly periodic subscriptions.
//
// Heuristic:
// - expense
// - >= N occurrences (default 3)
// - total >= minTotal
// - average <= maxAvg (focus on small but frequent)
func DetectLeaks(txs []Transaction, minCount int, minTotal float64, maxAvg float64) []LeakCandidate {
	if minCount <= 0 {
		minCount = 3
	}
	if minTotal <= 0 {
		minTotal = 15
	}
	if maxAvg <= 0 {
		maxAvg = 10
	}

	byName := map[string][]Transaction{}
	for _, tx := range txs {
		if tx.Classification != "expense" {
			continue
		}
		byName[tx.Name] = append(byName[tx.Name], tx)
	}

	var out []LeakCandidate
	for name, list := range byName {
		if len(list) < minCount {
			continue
		}
		var total float64
		var spike float64
		ids := make([]string, 0, min(3, len(list)))
		for i, tx := range list {
			v, err := SignedAmount(tx)
			if err != nil {
				continue
			}
			amt := math.Abs(v)
			total += amt
			if amt > spike {
				spike = amt
			}
			if i < 3 {
				ids = append(ids, tx.ID)
			}
		}
		avg := total / float64(len(list))
		if total < minTotal {
			continue
		}
		if avg > maxAvg {
			continue
		}

		conf := 0.6
		if len(list) >= 5 {
			conf += 0.15
		}
		if total >= 50 {
			conf += 0.1
		}
		if conf > 1 {
			conf = 1
		}

		out = append(out, LeakCandidate{
			Name:        name,
			Count:       len(list),
			TotalAmount: round2(total),
			AvgAmount:   round2(avg),
			SpikeAmount: round2(spike),
			SampleTxIDs: ids,
			Confidence:  conf,
			Reason:      "small_frequent_expenses",
		})
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Confidence == out[j].Confidence {
			return out[i].TotalAmount > out[j].TotalAmount
		}
		return out[i].Confidence > out[j].Confidence
	})
	return out
}
