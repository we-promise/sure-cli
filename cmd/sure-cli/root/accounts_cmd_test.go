package root

import "testing"

func TestAccountsListFlags(t *testing.T) {
	list := findSub(t, newAccountsCmd(), "list")
	for _, name := range []string{"page", "per-page", "include-disabled", "all"} {
		if list.Flags().Lookup(name) == nil {
			t.Fatalf("expected flag %q on accounts list", name)
		}
	}
	if def := list.Flags().Lookup("include-disabled").DefValue; def != "false" {
		t.Fatalf("include-disabled default = %q, want false (Sure hides disabled accounts by default)", def)
	}
}
