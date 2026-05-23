package root

import "testing"

func TestUsersResetCommandShape(t *testing.T) {
	cmd := newUsersCmd()
	reset, _, err := cmd.Find([]string{"reset"})
	if err != nil {
		t.Fatalf("find reset command: %v", err)
	}
	if reset.Flags().Lookup("apply") == nil {
		t.Fatal("expected reset --apply flag")
	}
	if _, _, err := cmd.Find([]string{"reset", "status"}); err != nil {
		t.Fatalf("find reset status command: %v", err)
	}
}

func TestUsersCommandRegistered(t *testing.T) {
	cmd := New()
	for _, args := range [][]string{
		{"users", "reset"},
		{"users", "reset", "status"},
		{"users", "delete-me"},
	} {
		if _, _, err := cmd.Find(args); err != nil {
			t.Fatalf("expected command %v: %v", args, err)
		}
	}
}

func TestUsersDestructiveCmds_RejectPositionalArgs(t *testing.T) {
	// reset, reset status, and delete-me must reject any positional argument
	// so a typo (e.g. `users reset --apply oops`) is caught instead of
	// silently running the destructive call.
	cmd := newUsersCmd()
	for _, path := range [][]string{
		{"reset", "extra"},
		{"reset", "status", "extra"},
		{"delete-me", "extra"},
	} {
		sub, _, err := cmd.Find(path[:len(path)-1])
		if err != nil {
			t.Fatalf("find %v: %v", path[:len(path)-1], err)
		}
		if err := sub.Args(sub, []string{path[len(path)-1]}); err == nil {
			t.Fatalf("%v: expected NoArgs to reject a positional argument", path)
		}
	}
}
