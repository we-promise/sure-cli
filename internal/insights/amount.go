package insights

import (
	"errors"
	"strconv"
	"strings"
	"unicode"
)

// ParseAmount parses formatted money strings such as "$112.00", "€1,23", or "-£2.00".
// Currency symbols and spaces are ignored; the returned value is numeric only.
func ParseAmount(s string) (float64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, errors.New("empty amount")
	}

	var b strings.Builder
	for _, r := range s {
		if unicode.IsDigit(r) || r == '-' || r == '+' || r == '.' || r == ',' {
			b.WriteRune(r)
		}
	}
	cleaned := b.String()
	if cleaned == "" || cleaned == "-" || cleaned == "+" {
		return 0, errors.New("empty amount")
	}

	neg := false
	if strings.HasPrefix(cleaned, "-") {
		neg = true
		cleaned = strings.TrimPrefix(cleaned, "-")
	} else {
		cleaned = strings.TrimPrefix(cleaned, "+")
	}

	if strings.Contains(cleaned, ",") && strings.Contains(cleaned, ".") {
		cleaned = strings.ReplaceAll(cleaned, ",", "")
	} else {
		cleaned = strings.ReplaceAll(cleaned, ",", ".")
	}

	v, err := strconv.ParseFloat(cleaned, 64)
	if err != nil {
		return 0, err
	}
	if neg {
		v = -v
	}
	return v, nil
}

// ParseAmountEUR parses Sure API amount strings like "€112.00", "-€2.00", "€1,23".
// Returns the numeric value as float64.
func ParseAmountEUR(s string) (float64, error) {
	return ParseAmount(s)
}

// SignedAmount normalizes to agent-friendly sign: expense negative, income positive.
// (Sure stores expenses as positive entries internally; API amount strings appear inverted vs UI.)
func SignedAmount(t Transaction) (float64, error) {
	v, err := ParseAmount(t.AmountText)
	if err != nil {
		return 0, err
	}
	// Classification is the ground truth for sign.
	if t.Classification == "income" {
		return abs(v), nil
	}
	if t.Classification == "expense" {
		return -abs(v), nil
	}
	return v, nil
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
