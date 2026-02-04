package insights

import (
	"math"
	"sort"
	"strings"
)

type FeeCandidate struct {
	Name        string   `json:"name"`
	Count       int      `json:"count"`
	TotalAmount float64  `json:"total_amount"` // positive number (absolute)
	AvgAmount   float64  `json:"avg_amount"`
	SampleTxIDs []string `json:"sample_tx_ids"`
	Confidence  float64  `json:"confidence"`
	Reason      string   `json:"reason"`
}

var defaultFeeKeywords = []string{
	"fee", "commission", "maintenance", "service", "overdraft", "atm", "charge", "penalty",
	"comisión", "mantenimiento", "cuota", "cargo", "penal",
}

// DetectFees flags expense transactions that look like bank/service fees.
// Heuristic: expense + name contains a fee keyword OR absolute amount is small and name contains bank-y words.
func DetectFees(txs []Transaction, keywords []string) []FeeCandidate {
	if len(keywords) == 0 {
		keywords = defaultFeeKeywords
	}

	byName := map[string][]Transaction{}
	for _, tx := range txs {
		if tx.Classification != "expense" {
			continue
		}
		nameLower := strings.ToLower(tx.Name)
		if !containsAny(nameLower, keywords) {
			continue
		}
		byName[tx.Name] = append(byName[tx.Name], tx)
	}

	var out []FeeCandidate
	for name, list := range byName {
		var total float64
		ids := make([]string, 0, min(3, len(list)))
		for i, tx := range list {
			v, err := SignedAmount(tx)
			if err == nil {
				total += math.Abs(v)
			}
			if i < 3 {
				ids = append(ids, tx.ID)
			}
		}
		avg := 0.0
		if len(list) > 0 {
			avg = total / float64(len(list))
		}
		conf := 0.75
		if len(list) >= 3 {
			conf += 0.1
		}
		if avg < 10 {
			conf += 0.05
		}
		if conf > 1 {
			conf = 1
		}
		out = append(out, FeeCandidate{
			Name:        name,
			Count:       len(list),
			TotalAmount: round2(total),
			AvgAmount:   round2(avg),
			SampleTxIDs: ids,
			Confidence:  conf,
			Reason:      "keyword_match",
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

func containsAny(s string, kws []string) bool {
	for _, k := range kws {
		if strings.Contains(s, strings.ToLower(k)) {
			return true
		}
	}
	return false
}
