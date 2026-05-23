package root

import (
	"testing"
)

func TestUsageCommandShape(t *testing.T) {
	cmd := newUsageCmd()
	if cmd.Use != "usage" {
		t.Fatalf("Use = %q, want usage", cmd.Use)
	}
	show := findSub(t, cmd, "show")
	if show.Args == nil {
		t.Fatal("usage show should reject extra args")
	}
}

func TestUsageCommandRegistered(t *testing.T) {
	root := New()
	cases := []struct {
		path []string
		want string
	}{
		{[]string{"usage"}, "usage"},
		{[]string{"usage", "show"}, "show"},
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
