package root

import "testing"

func TestBuildTagCreatePayload_RequiresName(t *testing.T) {
	if _, err := buildTagCreatePayload(tagWriteOpts{}); err == nil {
		t.Fatal("expected missing name error")
	}
	payload, err := buildTagCreatePayload(tagWriteOpts{Name: "Travel", Color: "#3b82f6"})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	tag, ok := payload["tag"].(map[string]any)
	if !ok {
		t.Fatalf("expected payload[\"tag\"] to be map[string]any, got %T: %#v", payload["tag"], payload)
	}
	if tag["name"] != "Travel" || tag["color"] != "#3b82f6" {
		t.Fatalf("unexpected tag payload: %#v", tag)
	}
}

func TestBuildTagCreatePayload_ColorOptional(t *testing.T) {
	payload, err := buildTagCreatePayload(tagWriteOpts{Name: "Travel"})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	tag := payload["tag"].(map[string]any)
	if _, has := tag["color"]; has {
		t.Fatalf("color should be omitted when empty, got %#v", tag)
	}
}

func TestBuildTagUpdatePayload_NeedsAtLeastOneField(t *testing.T) {
	if _, err := buildTagUpdatePayload(tagWriteOpts{}); err == nil {
		t.Fatal("expected error when no fields provided")
	}
}

func TestBuildTagUpdatePayload_PartialOK(t *testing.T) {
	payload, err := buildTagUpdatePayload(tagWriteOpts{Color: "#ff0000"})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	tag := payload["tag"].(map[string]any)
	if tag["color"] != "#ff0000" {
		t.Fatalf("color = %v", tag["color"])
	}
	if _, has := tag["name"]; has {
		t.Fatalf("name should be omitted, got %#v", tag)
	}
}

func TestReferenceCommandsRegistered(t *testing.T) {
	root := New()
	// cobra's Find returns the nearest matching ancestor when a leaf is
	// missing, so compare resolved Name to the expected leaf.
	cases := []struct {
		args []string
		want string
	}{
		{[]string{"categories", "list"}, "list"},
		{[]string{"categories", "show"}, "show"},
		{[]string{"categories", "create"}, "create"},
		{[]string{"merchants", "list"}, "list"},
		{[]string{"merchants", "show"}, "show"},
		{[]string{"tags", "create"}, "create"},
		{[]string{"tags", "update"}, "update"},
		{[]string{"tags", "delete"}, "delete"},
		{[]string{"rules", "list"}, "list"},
		{[]string{"rule-runs", "show"}, "show"},
	}
	for _, c := range cases {
		got, _, err := root.Find(c.args)
		if err != nil {
			t.Fatalf("path %v not registered: %v", c.args, err)
		}
		if got.Name() != c.want {
			t.Fatalf("path %v resolved to %q, want %q", c.args, got.Name(), c.want)
		}
	}
}
