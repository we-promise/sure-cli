package root

import "github.com/spf13/cobra"

// addAny sets key to value unless value is empty.
func addAny(m map[string]any, key, value string) {
	if value != "" {
		m[key] = value
	}
}

// emptyFlags returns which of the named flags were given as "". On an
// update that is how a user clears a field: Sure stores the empty value
// (and normalizes a blank payment_url to nil).
func emptyFlags(cmd *cobra.Command, names ...string) map[string]bool {
	empty := map[string]bool{}
	for _, name := range names {
		if f := cmd.Flags().Lookup(name); f != nil && f.Changed && f.Value.String() == "" {
			empty[name] = true
		}
	}
	return empty
}

// addCleared sends "" for each flag (flag name -> payload key) given empty.
func addCleared(m map[string]any, empty map[string]bool, fields map[string]string) {
	for flag, key := range fields {
		if empty[flag] {
			m[key] = ""
		}
	}
}
