package models

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// decimalRE matches a plain fixed-point decimal: an optional minus sign,
// digits, and an optional fraction. It is the shape Sure renders with
// BigDecimal#to_s("F") and the shape the CLI accepts for amounts it sends.
var decimalRE = regexp.MustCompile(`^-?[0-9]+(\.[0-9]+)?$`)

// IsDecimal reports whether s is a plain fixed-point decimal such as "12",
// "-0.5" or "12.3456" (no exponent, grouping, currency or plus sign).
func IsDecimal(s string) bool {
	return decimalRE.MatchString(s)
}

// ResolveSignedAmount is the single place that turns a Sure transaction's
// amount fields into the CLI's sign convention: income positive, expense
// negative. It returns the value and the same value as exact decimal text.
//
// amountDecimal is the response's `amount_decimal` (nil when the server
// predates it, Sure #4422): the stored decimal(19,4) value in the entry
// convention (positive = outflow), so it is negated textually and never
// rounded. Older servers only send `amount`, the localized display string,
// also in the entry convention but rounded to the currency's precision;
// it is parsed and signed from classification.
func ResolveSignedAmount(amountDecimal *string, display, classification string) (float64, string, error) {
	if amountDecimal != nil {
		text, err := negateDecimal(*amountDecimal)
		if err != nil {
			return 0, "", err
		}
		v, err := strconv.ParseFloat(text, 64)
		if err != nil {
			return 0, "", fmt.Errorf("amount_decimal %q: %w", *amountDecimal, err)
		}
		return v, text, nil
	}

	v, err := ParseAmount(display)
	if err != nil {
		return 0, "", fmt.Errorf("amount %q: %w", display, err)
	}
	switch classification {
	case "income":
		v = math.Abs(v)
	case "expense":
		v = -math.Abs(v)
	default:
		v = -v
	}
	if v == 0 {
		v = 0 // drop the sign of -0
	}
	return v, strconv.FormatFloat(v, 'f', -1, 64), nil
}

// negateDecimal flips the sign of a fixed-point decimal string without a
// float round trip. Zero keeps no sign.
func negateDecimal(s string) (string, error) {
	if !IsDecimal(s) {
		return "", fmt.Errorf("amount_decimal %q is not a fixed-point decimal", s)
	}
	if strings.HasPrefix(s, "-") {
		return s[1:], nil
	}
	if strings.Trim(s, "0.") == "" {
		return s, nil
	}
	return "-" + s, nil
}

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
