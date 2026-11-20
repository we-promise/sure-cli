package models

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/we-promise/sure-cli/internal/money"
)

// decimalRE is the CLI's one decimal grammar: an optional minus sign,
// digits, and an optional fraction with at least one digit. It is the shape
// Sure renders with BigDecimal#to_s("F"), and every amount the CLI sends to
// Sure must match it: BigDecimal() rejects "3.", and exponents, grouping,
// currency symbols and plus signs have no single reading.
var decimalRE = regexp.MustCompile(`^-?[0-9]+(\.[0-9]+)?$`)

// IsDecimal reports whether s is a plain fixed-point decimal such as "12",
// "-0.5" or "12.3456".
func IsDecimal(s string) bool {
	return decimalRE.MatchString(s)
}

// IsPositiveDecimal reports whether s is a decimal greater than zero.
func IsPositiveDecimal(s string) bool {
	return IsDecimal(s) && !strings.HasPrefix(s, "-") && strings.ContainsAny(s, "123456789")
}

// DecimalScale is the number of significant decimal places of a decimal:
// "12.3400" has 2, as BigDecimal("12.3400") does.
func DecimalScale(s string) int {
	_, frac, _ := strings.Cut(s, ".")
	return len(strings.TrimRight(frac, "0"))
}

// DecimalIntegerDigits counts the integer digits of a decimal, ignoring the
// sign and leading zeros: "-0012.5" has 2.
func DecimalIntegerDigits(s string) int {
	whole, _, _ := strings.Cut(strings.TrimPrefix(s, "-"), ".")
	return len(strings.TrimLeft(whole, "0"))
}

// AmountFields are the amount fields of a Sure transaction
// (api/v1/transactions/_transaction.json.jbuilder).
type AmountFields struct {
	// Decimal is `amount_decimal` (Sure #4422): the stored decimal(19,4)
	// value in the entry convention (positive = outflow). Nil on older servers.
	Decimal *string
	// SignedCents is `signed_amount_cents` (Sure #899): the amount rounded to
	// the currency's minor unit, income positive. Nil on older servers.
	SignedCents *json.Number
	// Display is `amount`, the localized display string in the entry
	// convention, rounded to the currency's precision.
	Display        string
	Currency       string
	Classification string
}

// ResolveSignedAmount is the single place that turns a Sure transaction's
// amount fields into the CLI's sign convention: income positive, expense
// negative. It returns the value and the same value as exact decimal text.
//
// It reads the most exact field the server sends: amount_decimal, else
// signed_amount_cents scaled by the currency's minor unit, else the display
// string, signed from classification.
func ResolveSignedAmount(f AmountFields) (float64, string, error) {
	text, err := signedAmountText(f)
	if err != nil {
		return 0, "", err
	}
	v, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return 0, "", err
	}
	return v, text, nil
}

func signedAmountText(f AmountFields) (string, error) {
	switch {
	case f.Decimal != nil:
		if !IsDecimal(*f.Decimal) {
			return "", fmt.Errorf("amount_decimal %q is not a fixed-point decimal", *f.Decimal)
		}
		return negateDecimal(*f.Decimal), nil
	case f.SignedCents != nil:
		text, err := money.MinorToDecimal(f.SignedCents.String(), f.Currency)
		if err != nil {
			return "", fmt.Errorf("signed_amount_cents: %w", err)
		}
		return text, nil
	}

	text, err := ParseDisplayAmount(f.Display, f.Currency)
	if err != nil {
		return "", fmt.Errorf("amount %q: %w", f.Display, err)
	}
	abs := strings.TrimPrefix(text, "-")
	switch f.Classification {
	case "income":
		return abs, nil
	case "expense":
		return negateDecimal(abs), nil
	default:
		return negateDecimal(text), nil
	}
}

// negateDecimal flips the sign of a decimal without a float round trip.
// Zero keeps no sign.
func negateDecimal(s string) string {
	if strings.HasPrefix(s, "-") {
		return s[1:]
	}
	if strings.Trim(s, "0.") == "" {
		return s
	}
	return "-" + s
}

// numberSpanRE finds the number in a display string: digit runs joined by
// single separators. It starts at the first digit, so symbols that carry
// dots of their own ("ر.س", "Rs.", "kr.") stay outside it.
var numberSpanRE = regexp.MustCompile("[0-9]+(?:[.,'\u00a0\u202f ][0-9]+)*")

// ParseDisplayAmount reads a money string as Sure's Money#format renders it
// ("€1,234.56", "1.234,56 €", "R$ 12.345,00", "¥1,234", "KD1.234") and
// returns it as exact decimal text in the IsDecimal grammar.
//
// The delimiter and separator depend on the server's locale, so they are
// inferred: spaces and apostrophes only group; a '.' or ',' that repeats
// groups; after any other separator, the last '.' or ',' is the decimal
// mark; otherwise a lone one is the decimal mark unless exactly three digits
// follow it and the currency does not show three decimals (money.Precision),
// which makes it a thousands group.
// A '-' anywhere before the number makes it negative.
func ParseDisplayAmount(display, currency string) (string, error) {
	loc := numberSpanRE.FindStringIndex(display)
	if loc == nil {
		return "", errors.New("no number in amount")
	}
	span := display[loc[0]:loc[1]]
	negative := strings.Contains(display[:loc[0]], "-")

	whole, frac := span, ""
	if i := strings.LastIndexAny(span, ".,"); i >= 0 {
		mark := span[i]
		after := len(span) - i - 1
		others := ",'\u00a0\u202f "
		if mark == ',' {
			others = ".'\u00a0\u202f "
		}
		repeated := strings.Count(span, string(mark)) > 1
		mixed := strings.ContainsAny(span[:i], others)
		if !repeated && (mixed || after != 3 || money.Precision(currency) == 3) {
			whole, frac = span[:i], span[i+1:]
		}
	}
	whole = strings.TrimLeft(stripNonDigits(whole), "0")
	if whole == "" {
		whole = "0"
	}
	text := whole
	if frac != "" {
		text += "." + frac
	}
	if negative {
		text = negateDecimal(text)
	}
	return text, nil
}

func stripNonDigits(s string) string {
	return strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, s)
}
