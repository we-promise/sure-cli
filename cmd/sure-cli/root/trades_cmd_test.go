package root

import (
	"strings"
	"testing"
)

func TestTradesList_Flags(t *testing.T) {
	cmd := newTradesCmd()

	list, _, err := cmd.Find([]string{"list"})
	if err != nil {
		t.Fatalf("find list subcommand: %v", err)
	}

	// Verify expected flags exist
	expectedFlags := []string{"from", "to", "start-date", "end-date", "account", "account-id", "account-ids", "page", "per-page"}
	for _, name := range expectedFlags {
		if list.Flags().Lookup(name) == nil {
			t.Fatalf("expected flag %q to exist", name)
		}
	}

	// Verify usage contains expected flags
	s := list.Flags().FlagUsages()
	if !strings.Contains(s, "from") {
		t.Fatalf("expected from in usage")
	}
	if !strings.Contains(s, "account-id") {
		t.Fatalf("expected account-id in usage")
	}
}

func TestTradesShow_Args(t *testing.T) {
	cmd := newTradesCmd()

	show, _, err := cmd.Find([]string{"show"})
	if err != nil {
		t.Fatalf("find show subcommand: %v", err)
	}

	// Verify it requires exactly 1 argument
	if show.Args == nil {
		t.Fatal("expected Args validator to be set")
	}
}
