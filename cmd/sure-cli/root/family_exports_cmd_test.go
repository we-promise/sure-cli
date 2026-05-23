package root

import "testing"

func TestFamilyExportsCommandShape(t *testing.T) {
	cmd := newFamilyExportsCmd()
	for _, args := range [][]string{
		{"list"},
		{"show"},
		{"create"},
		{"download"},
	} {
		if _, _, err := cmd.Find(args); err != nil {
			t.Fatalf("expected family-exports command %v: %v", args, err)
		}
	}

	create, _, err := cmd.Find([]string{"create"})
	if err != nil {
		t.Fatalf("find create command: %v", err)
	}
	if create.Flags().Lookup("apply") == nil {
		t.Fatal("expected create --apply flag")
	}

	download, _, err := cmd.Find([]string{"download"})
	if err != nil {
		t.Fatalf("find download command: %v", err)
	}
	if download.Flags().Lookup("out") == nil {
		t.Fatal("expected download --out flag")
	}
}
