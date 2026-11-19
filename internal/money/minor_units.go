// Package money converts Sure's integer minor-unit amounts (the *_cents
// fields) back to major units.
package money

import "strings"

// defaultMinorUnitConversion is Money::Currency#minor_unit_conversion for
// every currency Sure ships that has two decimals.
const defaultMinorUnitConversion = 100

// minorUnitExceptions lists every currency in Sure's config/currencies.yml
// whose minor_unit_conversion is not 100. Sure computes
// `*_cents = (amount * minor_unit_conversion).round`, so dividing by 100
// would misreport JPY by 100x, KWD by 10x, and so on.
var minorUnitExceptions = map[string]float64{
	"BHD": 1000, "BIF": 1, "BTC": 1e8, "BYR": 1, "CLF": 10000, "CLP": 1,
	"DJF": 1, "DOGE": 1e8, "ETH": 1e18, "GBX": 1, "GNF": 1, "HUF": 1,
	"IQD": 1000, "ISK": 1, "JOD": 1000, "JPY": 1, "KMF": 1, "KRW": 1,
	"KWD": 1000, "LYD": 1000, "MGA": 5, "MRU": 5, "OMR": 1000, "PYG": 1,
	"RWF": 1, "TND": 1000, "UGX": 1, "VND": 1, "VUV": 1, "XAF": 1,
	"XAG": 1, "XAU": 1, "XBA": 1, "XBB": 1, "XBC": 1, "XBD": 1,
	"XDR": 1, "XOF": 1, "XPD": 1, "XPF": 1, "XPT": 1, "XTS": 1,
}

// MinorUnitConversion returns how many minor units make one major unit of
// the ISO currency code (case-insensitive). Unknown codes use 100.
func MinorUnitConversion(code string) float64 {
	if c, ok := minorUnitExceptions[strings.ToUpper(strings.TrimSpace(code))]; ok {
		return c
	}
	return defaultMinorUnitConversion
}

// FromMinor converts a minor-unit amount to major units of code.
func FromMinor(minor float64, code string) float64 {
	return minor / MinorUnitConversion(code)
}
