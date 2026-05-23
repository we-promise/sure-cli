package root

import "testing"

func TestBuildCategoryCreatePayload_RequiresName(t *testing.T) {
	if _, err := buildCategoryCreatePayload(categoryCreateOpts{Color: "#3b82f6"}); err == nil {
		t.Fatal("expected missing name to error")
	}
}

func TestBuildCategoryCreatePayload_RequiresColor(t *testing.T) {
	if _, err := buildCategoryCreatePayload(categoryCreateOpts{Name: "Food"}); err == nil {
		t.Fatal("expected missing color to error (Category model validates color presence)")
	}
}

func TestBuildCategoryCreatePayload_ColorMustBeHex(t *testing.T) {
	// Upstream validates `format: { with: /\A#[0-9A-Fa-f]{6}\z/ }`. Catch the
	// obvious format errors client-side so users get fast feedback.
	cases := []string{"3b82f6", "blue", "#abc", "#GGGGGG", "#3b82f6 "}
	for _, c := range cases {
		if _, err := buildCategoryCreatePayload(categoryCreateOpts{Name: "Food", Color: c}); err == nil {
			t.Fatalf("expected color %q to be rejected", c)
		}
	}
}

func TestBuildCategoryCreatePayload_AcceptsValidHex(t *testing.T) {
	for _, c := range []string{"#3b82f6", "#000000", "#FFFFFF", "#abcdef"} {
		if _, err := buildCategoryCreatePayload(categoryCreateOpts{Name: "Food", Color: c}); err != nil {
			t.Fatalf("color %q should be accepted, got %v", c, err)
		}
	}
}

func TestBuildCategoryCreatePayload_WrapsInCategoryKey(t *testing.T) {
	// Upstream uses `params.require(:category)` — body must be `{"category": {...}}`.
	payload, err := buildCategoryCreatePayload(categoryCreateOpts{Name: "Food", Color: "#3b82f6"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	cat, ok := payload["category"].(map[string]any)
	if !ok {
		t.Fatalf("payload['category'] not map: %#v", payload)
	}
	if cat["name"] != "Food" || cat["color"] != "#3b82f6" {
		t.Fatalf("category = %#v", cat)
	}
	if _, has := cat["icon"]; has {
		t.Fatal("icon should be omitted when empty (upstream auto-suggests)")
	}
	if _, has := cat["parent_id"]; has {
		t.Fatal("parent_id should be omitted when empty")
	}
}

func TestBuildCategoryCreatePayload_OptionalFields(t *testing.T) {
	payload, err := buildCategoryCreatePayload(categoryCreateOpts{
		Name:     "Subscriptions",
		Color:    "#3b82f6",
		Icon:     "wallet",
		ParentID: "parent-uuid",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	cat := payload["category"].(map[string]any)
	if cat["icon"] != "wallet" {
		t.Fatalf("icon = %v", cat["icon"])
	}
	if cat["parent_id"] != "parent-uuid" {
		t.Fatalf("parent_id = %v", cat["parent_id"])
	}
}

func TestBuildCategoryCreatePayload_WhitespaceOnlyNameRejected(t *testing.T) {
	if _, err := buildCategoryCreatePayload(categoryCreateOpts{Name: "   ", Color: "#3b82f6"}); err == nil {
		t.Fatal("expected whitespace-only name to be rejected")
	}
}

func TestBuildCategoryCreatePayload_TrimsNameInPayload(t *testing.T) {
	// Regression: trimming was applied for validation but the original value
	// was sent in the payload, leaking whitespace into the upstream uniqueness
	// check.
	payload, err := buildCategoryCreatePayload(categoryCreateOpts{Name: "  Food  ", Color: "#3b82f6"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	cat := payload["category"].(map[string]any)
	if cat["name"] != "Food" {
		t.Fatalf("name in payload not trimmed: %q", cat["name"])
	}
}

func TestCategoriesCreateRegistered(t *testing.T) {
	root := New()
	got, _, err := root.Find([]string{"categories", "create"})
	if err != nil {
		t.Fatalf("categories create not registered: %v", err)
	}
	if got.Name() != "create" {
		t.Fatalf("resolved to %q, want create", got.Name())
	}
	for _, f := range []string{"name", "color", "icon", "parent-id", "apply"} {
		if got.Flags().Lookup(f) == nil {
			t.Fatalf("categories create missing --%s", f)
		}
	}
}
