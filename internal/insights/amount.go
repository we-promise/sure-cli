package insights

import (
	"errors"
	"strconv"
	"strings"
)

// ParseAmountEUR parses Sure API amount strings like "€112.00", "-€2.00", "€1,23".
// Returns the numeric value as float64.
func ParseAmountEUR(s string) (float64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, errors.New("empty amount")
	}

	neg := false
	if strings.HasPrefix(s, "-") {
		neg = true
		s = strings.TrimPrefix(s, "-")
	}
	// remove currency symbol and spaces
	s = strings.ReplaceAll(s, "€", "")
	s = strings.ReplaceAll(s, " ", "")
	// Handle separators:
	// - If both ',' and '.' present, assume ',' is thousands separator (US style: 2,000.00) -> remove commas.
	// - If only ',' present, assume decimal comma -> replace with '.'
	if strings.Contains(s, ",") && strings.Contains(s, ".") {
		s = strings.ReplaceAll(s, ",", "")
	} else {
		s = strings.ReplaceAll(s, ",", ".")
	}

	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, err
	}
	if neg {
		v = -v
	}
	return v, nil
}

// SignedAmount normalizes to agent-friendly sign: expense negative, income positive.
// (Sure stores expenses as positive entries internally; API amount strings appear inverted vs UI.)
func SignedAmount(t Transaction) (float64, error) {
	v, err := ParseAmountEUR(t.AmountText)
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
