package root

import "testing"

func TestBuildTxCreatePayload_Valid(t *testing.T) {
	p, err := buildTxCreatePayload(txCreateOpts{
		AccountID: "acc",
		Date:      "2026-02-04",
		Amount:    1.23,
		Nature:    "expense",
		Name:      "coffee",
	})
	if err != nil {
		t.Fatalf("expected nil err, got %v", err)
	}
	tx := p["transaction"].(map[string]any)
	if tx["account_id"] != "acc" {
		t.Fatalf("account_id mismatch")
	}
	if tx["amount"] != "1.23" {
		t.Fatalf("amount mismatch: %v", tx["amount"])
	}
}

func TestBuildTxCreatePayload_RequiresAccount(t *testing.T) {
	_, err := buildTxCreatePayload(txCreateOpts{Name: "x", Amount: 1, Nature: "expense", Date: "2026-02-04"})
	if err == nil {
		t.Fatalf("expected err")
	}
}

func TestBuildTxCreatePayload_RequiresName(t *testing.T) {
	_, err := buildTxCreatePayload(txCreateOpts{AccountID: "a", Amount: 1, Nature: "expense", Date: "2026-02-04"})
	if err == nil {
		t.Fatalf("expected err")
	}
}

func TestBuildTxCreatePayload_RequiresNonZeroAmount(t *testing.T) {
	_, err := buildTxCreatePayload(txCreateOpts{AccountID: "a", Name: "x", Amount: 0, Nature: "expense", Date: "2026-02-04"})
	if err == nil {
		t.Fatalf("expected err")
	}
}

func TestBuildTxCreatePayload_ValidatesDate(t *testing.T) {
	_, err := buildTxCreatePayload(txCreateOpts{AccountID: "a", Name: "x", Amount: 1, Nature: "expense", Date: "nope"})
	if err == nil {
		t.Fatalf("expected err")
	}
}

func TestBuildTxCreatePayload_ValidatesNature(t *testing.T) {
	_, err := buildTxCreatePayload(txCreateOpts{AccountID: "a", Name: "x", Amount: 1, Nature: "weird", Date: "2026-02-04"})
	if err == nil {
		t.Fatalf("expected err")
	}
}
