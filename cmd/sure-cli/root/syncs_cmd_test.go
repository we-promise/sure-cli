package root

import (
	"testing"
)

func TestSyncsCommandShape(t *testing.T) {
	cmd := newSyncsCmd()
	if cmd.Use != "syncs" {
		t.Fatalf("Use = %q, want syncs", cmd.Use)
	}

	list := findSub(t, cmd, "list")
	for _, name := range []string{"page", "per-page"} {
		if list.Flags().Lookup(name) == nil {
			t.Fatalf("expected flag %q on syncs list", name)
		}
	}

	latest := findSub(t, cmd, "latest")
	if latest.Args == nil {
		t.Fatal("syncs latest should reject extra args")
	}

	show := findSub(t, cmd, "show")
	if show.Args == nil {
		t.Fatal("syncs show should require an id")
	}
}

func TestSyncsCommandRegistered(t *testing.T) {
	root := New()
	cases := []struct {
		path []string
		want string
	}{
		{[]string{"syncs"}, "syncs"},
		{[]string{"syncs", "list"}, "list"},
		{[]string{"syncs", "latest"}, "latest"},
		{[]string{"syncs", "show"}, "show"},
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
