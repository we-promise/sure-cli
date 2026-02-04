package root

import (
	"strings"
	"testing"
)

func TestTransactionsList_QueryString(t *testing.T) {
	cmd := newTransactionsCmd()

	list, _, err := cmd.Find([]string{"list"})
	if err != nil {
		t.Fatalf("find list subcommand: %v", err)
	}

	// build flags
	_ = list.Flags().Set("from", "2026-01-01")
	_ = list.Flags().Set("to", "2026-01-31")
	_ = list.Flags().Set("account", "1")
	_ = list.Flags().Set("limit", "10")

	// We don't call Run (would hit network), but we can ensure flags exist and are set.
	// This test mainly guards against accidental flag removal/renaming.
	for _, name := range []string{"from", "to", "account", "category", "merchant", "limit"} {
		if list.Flags().Lookup(name) == nil {
			t.Fatalf("expected flag %q to exist", name)
		}
	}

	s := list.Flags().FlagUsages()
	if !strings.Contains(s, "from") {
		t.Fatalf("expected from in usage")
	}
}
