package insights

import (
	"math"
	"sort"
	"strings"
)

type FeeCandidate struct {
	Name            string   `json:"name"`
	Count           int      `json:"count"`
	TotalAmount     float64  `json:"total_amount"` // positive number (absolute)
	AvgAmount       float64  `json:"avg_amount"`
	SampleTxIDs     []string `json:"sample_tx_ids"`
	Confidence      float64  `json:"confidence"`
	Reason          string   `json:"reason"`
	SuggestedAction string   `json:"suggested_action"`
}

// DefaultFeeKeywords is the comprehensive list of fee-related keywords (EN + ES + common bank terms).
// Can be overridden via config or CLI flags.
var DefaultFeeKeywords = []string{
	// English - general
	"fee", "fees", "charge", "charges", "commission", "commissions",
	"penalty", "penalties", "fine", "fines",
	"surcharge", "markup", "premium",
	// English - banking
	"overdraft", "nsf", "insufficient funds",
	"maintenance", "account maintenance", "monthly fee", "annual fee",
	"atm", "atm fee", "atm withdrawal", "foreign atm",
	"wire", "wire fee", "transfer fee", "swift",
	"foreign transaction", "fx fee", "currency conversion",
	"card replacement", "statement fee", "paper statement",
	"inactivity", "dormant", "minimum balance",
	"returned item", "returned check", "bounced",
	"late fee", "late payment", "interest charge",
	"cash advance", "cash advance fee",
	// English - services
	"service fee", "service charge", "convenience fee",
	"processing fee", "handling fee", "admin fee", "administrative",
	"subscription fee", "membership fee",
	// Spanish - general
	"comisión", "comisiones", "cargo", "cargos", "recargo", "recargos",
	"penalización", "penalizacion", "multa", "multas",
	"cuota", "cuotas", "tarifa", "tarifas",
	// Spanish - banking
	"descubierto", "sobregiro", "números rojos",
	"mantenimiento", "mantenimiento cuenta", "cuota mensual", "cuota anual",
	"cajero", "cajero automático", "reintegro cajero",
	"transferencia", "comisión transferencia", "swift",
	"cambio divisa", "comisión cambio", "tipo de cambio",
	"reposición tarjeta", "extracto", "extracto papel",
	"inactividad", "saldo mínimo",
	"devolución", "cheque devuelto", "recibo devuelto",
	"demora", "pago atrasado", "intereses",
	"anticipo", "disposición efectivo",
	// Spanish - services
	"gastos", "gastos de gestión", "gastos administrativos",
	"comisión servicio", "comisión apertura", "comisión cancelación",
	// German (common in EU)
	"gebühr", "gebuhr", "kontoführung", "kontofuhrung",
	// French (common in EU)
	"frais", "commission", "agios",
}

// GetFeeKeywords returns keywords from config or defaults
func GetFeeKeywords(custom []string) []string {
	if len(custom) > 0 {
		return custom
	}
	return DefaultFeeKeywords
}

// DetectFees flags expense transactions that look like bank/service fees.
// Heuristic: expense + name contains a fee keyword OR absolute amount is small and name contains bank-y words.
func DetectFees(txs []Transaction, keywords []string) []FeeCandidate {
	keywords = GetFeeKeywords(keywords)

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
		action := "Check if avoidable"
		if total > 50 {
			action = "Contact bank to waive or reduce; consider switching accounts"
		} else if avg < 5 && len(list) >= 3 {
			action = "Small recurring fee; check if bundled in account package"
		}

		out = append(out, FeeCandidate{
			Name:            name,
			Count:           len(list),
			TotalAmount:     round2(total),
			AvgAmount:       round2(avg),
			SampleTxIDs:     ids,
			Confidence:      conf,
			Reason:          "keyword_match",
			SuggestedAction: action,
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
