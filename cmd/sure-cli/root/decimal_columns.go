package root

import (
	"fmt"
	"math"

	"github.com/we-promise/sure-cli/internal/models"
)

// decimalColumn is the Sure numeric column a decimal flag lands in: the
// decimals it keeps (Postgres rounds the rest away silently) and the integer
// digits it holds (more is an error).
type decimalColumn struct{ Scale, IntegerDigits int }

var (
	// decimal(19,4): entries.amount, trades.fee, security_prices.price.
	moneyColumn = decimalColumn{Scale: 4, IntegerDigits: 15}
	// trades.qty is decimal(34,18).
	tradeQtyColumn = decimalColumn{Scale: 18, IntegerDigits: 16}
	// trades.price is decimal(19,10).
	tradePriceColumn = decimalColumn{Scale: 10, IntegerDigits: 9}
	// queryBound is a filter value Sure compares as a number (to_f) and does
	// not store, so any decimal fits.
	queryBound = decimalColumn{Scale: math.MaxInt, IntegerDigits: math.MaxInt}
)

// check reports why the decimal s, the value of flag name, does not fit.
func (c decimalColumn) check(name, s string) error {
	if models.DecimalScale(s) > c.Scale {
		return fmt.Errorf("--%s has more than %d decimal places, got %q", name, c.Scale, s)
	}
	if models.DecimalIntegerDigits(s) > c.IntegerDigits {
		return fmt.Errorf("--%s has more than %d digits before the decimal point, got %q", name, c.IntegerDigits, s)
	}
	return nil
}

// requirePositiveDecimal checks a required positive decimal flag that lands
// in column.
func requirePositiveDecimal(name, value string, column decimalColumn) error {
	if value == "" {
		return fmt.Errorf("--%s is required", name)
	}
	if !models.IsPositiveDecimal(value) {
		return fmt.Errorf("--%s must be a positive decimal number, got %q", name, value)
	}
	return column.check(name, value)
}

// checkDecimal checks a signed decimal flag that lands in column.
func checkDecimal(name, value string, column decimalColumn) error {
	if !models.IsDecimal(value) {
		return fmt.Errorf("--%s must be a decimal number, got %q", name, value)
	}
	return column.check(name, value)
}

// checkOptionalDecimal is checkDecimal for a flag that may be left unset.
func checkOptionalDecimal(name, value string, column decimalColumn) error {
	if value == "" {
		return nil
	}
	return checkDecimal(name, value, column)
}
