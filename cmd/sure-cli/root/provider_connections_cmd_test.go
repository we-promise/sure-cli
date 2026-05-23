package root

import (
	"testing"
)

func TestProviderConnectionsCommandShape(t *testing.T) {
	cmd := newProviderConnectionsCmd()
	if cmd.Use != "provider-connections" {
		t.Fatalf("Use = %q, want provider-connections", cmd.Use)
	}

	list := findSub(t, cmd, "list")
	if list.Args == nil {
		t.Fatal("provider-connections list should reject extra args")
	}
	if list.Short == "" {
		t.Fatal("list should have a Short description")
	}
}

func TestProviderConnectionsRegistered(t *testing.T) {
	root := New()
	cases := []struct {
		path []string
		want string
	}{
		{[]string{"provider-connections"}, "provider-connections"},
		{[]string{"provider-connections", "list"}, "list"},
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
