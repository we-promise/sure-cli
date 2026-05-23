package root

import (
	"testing"

	"github.com/spf13/cobra"
)

func findSub(t *testing.T, cmd *cobra.Command, name string) *cobra.Command {
	t.Helper()
	sub, _, err := cmd.Find([]string{name})
	if err != nil {
		t.Fatalf("find %q: %v", name, err)
	}
	return sub
}

func TestTransfersCommandShape(t *testing.T) {
	cmd := newTransfersCmd()
	if cmd.Use != "transfers" {
		t.Fatalf("Use = %q, want transfers", cmd.Use)
	}

	list := findSub(t, cmd, "list")
	for _, name := range []string{"status", "account-id", "start-date", "end-date", "page", "per-page"} {
		if list.Flags().Lookup(name) == nil {
			t.Fatalf("expected flag %q on transfers list", name)
		}
	}

	show := findSub(t, cmd, "show")
	if show.Args == nil {
		t.Fatal("expected Args validator on transfers show")
	}
}

func TestRejectedTransfersCommandShape(t *testing.T) {
	cmd := newRejectedTransfersCmd()
	if cmd.Use != "rejected-transfers" {
		t.Fatalf("Use = %q, want rejected-transfers", cmd.Use)
	}

	list := findSub(t, cmd, "list")
	for _, name := range []string{"account-id", "start-date", "end-date", "page", "per-page"} {
		if list.Flags().Lookup(name) == nil {
			t.Fatalf("expected flag %q on rejected-transfers list", name)
		}
	}
	if list.Flags().Lookup("status") != nil {
		t.Fatal("rejected-transfers list must not expose --status (RejectedTransfer has no status column)")
	}

	show := findSub(t, cmd, "show")
	if show.Args == nil {
		t.Fatal("expected Args validator on rejected-transfers show")
	}
}

func TestTransfersCommandsRegistered(t *testing.T) {
	root := New()
	// cobra's Find silently returns the nearest matching ancestor when a leaf
	// is missing, so compare the resolved cmd's Name to the expected leaf.
	cases := []struct {
		path []string
		want string
	}{
		{[]string{"transfers"}, "transfers"},
		{[]string{"transfers", "list"}, "list"},
		{[]string{"transfers", "show"}, "show"},
		{[]string{"rejected-transfers"}, "rejected-transfers"},
		{[]string{"rejected-transfers", "list"}, "list"},
		{[]string{"rejected-transfers", "show"}, "show"},
	}
	for _, c := range cases {
		got, _, err := root.Find(c.path)
		if err != nil {
			t.Fatalf("path %v not registered: %v", c.path, err)
		}
		if got.Name() != c.want {
			t.Fatalf("path %v resolved to %q, want %q", c.path, got.Name(), c.want)
		}
	}
}
