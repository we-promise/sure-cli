package root

import (
	"testing"
)

func TestAuthCommandShape(t *testing.T) {
	cmd := newAuthCmd()
	if cmd.Use != "auth" {
		t.Fatalf("Use = %q", cmd.Use)
	}

	enableAI := findSub(t, cmd, "enable-ai")
	if enableAI.Args == nil {
		t.Fatal("auth enable-ai should reject extra args")
	}
	if enableAI.Flags().Lookup("apply") == nil {
		t.Fatal("auth enable-ai missing --apply (this is a write op)")
	}
}

func TestAuthRegistered(t *testing.T) {
	root := New()
	// cobra's Find returns the nearest matching ancestor with no error if a
	// leaf is missing, so we must compare the resolved cmd's Name to confirm
	// the actual subcommand is registered.
	cases := []struct {
		path []string
		want string
	}{
		{[]string{"auth"}, "auth"},
		{[]string{"auth", "enable-ai"}, "enable-ai"},
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
