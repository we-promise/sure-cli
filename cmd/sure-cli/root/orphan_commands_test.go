package root

import "testing"

// TestOrphanCommandsRegistered locks in registration for top-level commands
// that previously had no test file at all. Deleting the AddCommand line for
// any of these in root.go would silently still compile; this table makes
// such regressions fail fast.
//
// Tracks the test-review finding: whoami / sync / refresh / transactions
// delete / insights_* / plan_* were orphans.
func TestOrphanCommandsRegistered(t *testing.T) {
	root := New()
	cases := []struct {
		path []string
		want string
	}{
		{[]string{"whoami"}, "whoami"},
		{[]string{"sync"}, "sync"},
		{[]string{"refresh"}, "refresh"},
		{[]string{"login"}, "login"},
		{[]string{"status"}, "status"},
		{[]string{"export"}, "export"},
		{[]string{"export", "transactions"}, "transactions"},
		{[]string{"transactions", "delete"}, "delete"},
		{[]string{"insights"}, "insights"},
		{[]string{"insights", "subscriptions"}, "subscriptions"},
		{[]string{"insights", "fees"}, "fees"},
		{[]string{"insights", "leaks"}, "leaks"},
		{[]string{"plan"}, "plan"},
		{[]string{"plan", "budget"}, "budget"},
		{[]string{"plan", "runway"}, "runway"},
		{[]string{"plan", "forecast"}, "forecast"},
		{[]string{"propose"}, "propose"},
		{[]string{"propose", "rules"}, "rules"},
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
