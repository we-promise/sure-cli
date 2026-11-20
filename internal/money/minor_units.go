// Package money knows the minor units of the currencies Sure ships
// (config/currencies.yml): how its integer *_cents fields scale, and how
// many decimals Sure shows for each currency.
package money

import (
	"fmt"
	"math/big"
	"strings"
)

// unit is a currency's minor_unit_conversion and default_precision.
type unit struct {
	conversion int64
	precision  int
}

// defaultUnit is every currency Sure ships with two decimals.
var defaultUnit = unit{100, 2}

// unitExceptions lists every currency in Sure's config/currencies.yml whose
// minor_unit_conversion is not 100 or whose default_precision is not 2. Sure
// computes `*_cents = (amount * minor_unit_conversion).round`, so dividing
// by 100 would misreport JPY by 100x, KWD by 10x, and so on.
var unitExceptions = map[string]unit{
	"BHD": {1000, 3}, "BIF": {1, 0}, "BTC": {1e8, 8}, "BYR": {1, 0}, "CLF": {10000, 4},
	"CLP": {1, 0}, "DJF": {1, 0}, "DOGE": {1e8, 8}, "ETH": {1e18, 8}, "GBX": {1, 0},
	"GNF": {1, 0}, "HUF": {1, 0}, "IQD": {1000, 3}, "ISK": {1, 0}, "JOD": {1000, 3},
	"JPY": {1, 0}, "KMF": {1, 0}, "KRW": {1, 0}, "KWD": {1000, 3}, "LYD": {1000, 3},
	"MGA": {5, 1}, "MRU": {5, 1}, "OMR": {1000, 3}, "PYG": {1, 0}, "RWF": {1, 0},
	"TND": {1000, 3}, "UGX": {1, 0}, "VND": {1, 0}, "VUV": {1, 0}, "XAF": {1, 0},
	"XAG": {1, 0}, "XAU": {1, 0}, "XBA": {1, 0}, "XBB": {1, 0}, "XBC": {1, 0},
	"XBD": {1, 0}, "XDR": {1, 0}, "XOF": {1, 0}, "XPD": {1, 0}, "XPF": {1, 0},
	"XPT": {1, 0}, "XTS": {1, 0},
}

// unitOf looks up an ISO currency code case-insensitively, as
// Money::Currency.new does. Unknown codes get the two-decimal default.
func unitOf(code string) unit {
	if u, ok := unitExceptions[strings.ToUpper(strings.TrimSpace(code))]; ok {
		return u
	}
	return defaultUnit
}

// MinorUnitConversion returns how many minor units make one major unit of
// the currency.
func MinorUnitConversion(code string) float64 {
	return float64(unitOf(code).conversion)
}

// Precision returns how many decimals Sure shows for the currency
// (default_precision): 2 for EUR, 0 for JPY, 3 for KWD, 8 for BTC and ETH.
func Precision(code string) int {
	return unitOf(code).precision
}

// FromMinor converts a minor-unit amount to major units of code.
func FromMinor(minor float64, code string) float64 {
	return minor / MinorUnitConversion(code)
}

// MinorToDecimal converts an integer minor-unit amount, as JSON text, to the
// exact major-unit decimal ("-2050" USD is "-20.5"), with no float on the
// way: ETH counts 1e18 wei to the ether, beyond int64 and float64.
func MinorToDecimal(minor, code string) (string, error) {
	n, ok := new(big.Int).SetString(minor, 10)
	if !ok {
		return "", fmt.Errorf("minor-unit amount %q is not an integer", minor)
	}
	conversion := big.NewInt(unitOf(code).conversion)
	// The fewest decimals that write n/conversion exactly: every conversion
	// Sure ships divides a power of ten.
	digits, pow := 0, big.NewInt(1)
	for new(big.Int).Mod(pow, conversion).Sign() != 0 {
		pow.Mul(pow, big.NewInt(10))
		digits++
	}
	text := new(big.Rat).SetFrac(n, conversion).FloatString(digits)
	if strings.Contains(text, ".") {
		text = strings.TrimRight(strings.TrimRight(text, "0"), ".")
	}
	if text == "-0" {
		text = "0"
	}
	return text, nil
}
