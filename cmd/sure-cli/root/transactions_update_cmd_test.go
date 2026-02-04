package root

import "testing"

func TestBuildTxUpdatePayload_RequiresAtLeastOneField(t *testing.T) {
	_, err := buildTxUpdatePayload(txUpdateOpts{})
	if err == nil {
		t.Fatalf("expected err")
	}
}

func TestBuildTxUpdatePayload_ValidAmountRequiresNature(t *testing.T) {
	_, err := buildTxUpdatePayload(txUpdateOpts{Amount: 1, Date: "2026-02-04"})
	if err == nil {
		t.Fatalf("expected err")
	}
}

func TestBuildTxUpdatePayload_Valid(t *testing.T) {
	p, err := buildTxUpdatePayload(txUpdateOpts{Name: "x", Amount: 1.2, Nature: "expense", Date: "2026-02-04"})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	tx := p["transaction"].(map[string]any)
	if tx["amount"] != "1.20" {
		t.Fatalf("amount mismatch")
	}
	if tx["nature"] != "expense" {
		t.Fatalf("nature mismatch")
	}
}
