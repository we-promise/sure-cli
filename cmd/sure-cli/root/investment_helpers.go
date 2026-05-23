package root

import (
	"net/url"
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
