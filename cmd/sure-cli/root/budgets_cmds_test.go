package root

import "testing"

func TestBudgetsCommandShape(t *testing.T) {
	cmd := newBudgetsCmd()
	for _, args := range [][]string{{"list"}, {"show"}} {
		sub, _, err := cmd.Find(args)
		if err != nil {
			t.Fatalf("find budgets %v: %v", args, err)
		}
		if sub.Args == nil {
			t.Fatalf("budgets %v: expected Args validator", args)
		}
	}

	list, _, _ := cmd.Find([]string{"list"})
	for _, name := range []string{"page", "per-page", "start-date", "end-date"} {
		if list.Flags().Lookup(name) == nil {
			t.Fatalf("budgets list: expected flag %q", name)
		}
	}
}

func TestBudgetCategoriesCommandShape(t *testing.T) {
	cmd := newBudgetCategoriesCmd()
	for _, args := range [][]string{{"list"}, {"show"}} {
		sub, _, err := cmd.Find(args)
		if err != nil {
			t.Fatalf("find budget-categories %v: %v", args, err)
		}
		if sub.Args == nil {
			t.Fatalf("budget-categories %v: expected Args validator", args)
		}
	}

	list, _, _ := cmd.Find([]string{"list"})
	for _, name := range []string{"page", "per-page", "budget-id", "category-id", "start-date", "end-date"} {
		if list.Flags().Lookup(name) == nil {
			t.Fatalf("budget-categories list: expected flag %q", name)
		}
	}
}

func TestBudgetsCommandsRegistered(t *testing.T) {
	cmd := New()
	// cobra's Find silently returns the nearest matching ancestor when a leaf
	// is missing, so compare the resolved cmd's Name to the expected leaf.
	cases := []struct {
		args []string
		want string
	}{
		{[]string{"budgets", "list"}, "list"},
		{[]string{"budgets", "show"}, "show"},
		{[]string{"budget-categories", "list"}, "list"},
		{[]string{"budget-categories", "show"}, "show"},
	}
	for _, c := range cases {
		got, _, err := cmd.Find(c.args)
		if err != nil {
			t.Fatalf("expected command %v: %v", c.args, err)
		}
		if got.Name() != c.want {
			t.Fatalf("path %v resolved to %q, want %q", c.args, got.Name(), c.want)
		}
	}
}
