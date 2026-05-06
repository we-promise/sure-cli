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
