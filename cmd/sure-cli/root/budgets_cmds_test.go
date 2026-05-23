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
	for _, args := range [][]string{
		{"budgets", "list"},
		{"budgets", "show"},
		{"budget-categories", "list"},
		{"budget-categories", "show"},
	} {
		if _, _, err := cmd.Find(args); err != nil {
			t.Fatalf("expected command %v: %v", args, err)
		}
	}
}
