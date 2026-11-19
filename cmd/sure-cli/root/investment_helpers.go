package root

import (
	"net/url"
	"regexp"
	"strings"
)

// addRepeatedQuery appends q[key][]=v for each non-empty v. Used by trades
// and holdings commands that accept --account-ids etc. Lives here for
// historical reasons; safe to call from any command file.
func addRepeatedQuery(q url.Values, key string, values []string) {
	for _, v := range values {
		if v != "" {
			q.Add(key+"[]", v)
		}
	}
}

// decimalTextRE accepts plain decimal text ("12", "-0.5", ".25", "3.").
// Amounts travel to Sure as this exact text, never through a float.
var decimalTextRE = regexp.MustCompile(`^-?(\d+(\.\d*)?|\.\d+)$`)

func isDecimalText(s string) bool {
	return decimalTextRE.MatchString(s)
}

func isPositiveDecimalText(s string) bool {
	return isDecimalText(s) && !strings.HasPrefix(s, "-") && strings.ContainsAny(s, "123456789")
}

// decimalScale is the number of significant decimal places in decimal text:
// "12.3400" has 2, as BigDecimal("12.3400") does.
func decimalScale(s string) int {
	_, frac, ok := strings.Cut(s, ".")
	if !ok {
		return 0
	}
	return len(strings.TrimRight(frac, "0"))
}

// decimalIntegerDigits counts the integer digits of decimal text, ignoring
// sign and leading zeros: "-0012.5" has 2.
func decimalIntegerDigits(s string) int {
	whole, _, _ := strings.Cut(strings.TrimPrefix(s, "-"), ".")
	return len(strings.TrimLeft(whole, "0"))
}
