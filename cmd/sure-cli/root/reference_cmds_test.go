package root

import "testing"

func TestBuildTagPayload_CreateRequiresName(t *testing.T) {
	if _, err := buildTagPayload(tagWriteOpts{}, true); err == nil {
		t.Fatal("expected missing name error")
	}
	payload, err := buildTagPayload(tagWriteOpts{Name: "Travel", Color: "#3b82f6"}, true)
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

func TestReferenceCommandsRegistered(t *testing.T) {
	cmd := New()
	for _, args := range [][]string{
		{"categories", "list"},
		{"merchants", "show"},
		{"tags", "create"},
		{"rules", "list"},
		{"rule-runs", "show"},
	} {
		if _, _, err := cmd.Find(args); err != nil {
			t.Fatalf("expected command %v: %v", args, err)
		}
	}
}
