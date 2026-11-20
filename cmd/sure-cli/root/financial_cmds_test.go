package root

import (
	"strings"
	"testing"
)

func TestBuildValuationCreatePayloadUpsert(t *testing.T) {
	payload, err := buildValuationCreatePayload(valuationCreateOpts{
		AccountID: "acc_123",
		Amount:    "123.45",
		Date:      "2026-05-01",
		Notes:     "month end",
		Upsert:    true,
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if payload["upsert"] != true {
		t.Fatalf("expected upsert=true")
	}
	valuation := payload["valuation"].(map[string]any)
	if valuation["account_id"] != "acc_123" || valuation["amount"] != "123.45" {
		t.Fatalf("unexpected valuation payload: %#v", valuation)
	}
}

func TestBuildValuationUpdatePayloadRequiresAmountAndDateTogether(t *testing.T) {
	if _, err := buildValuationUpdatePayload(valuationUpdateOpts{Amount: "1.23"}); err == nil {
		t.Fatal("expected missing date error")
	}
	payload, err := buildValuationUpdatePayload(valuationUpdateOpts{Notes: "only notes"})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	valuation := payload["valuation"].(map[string]any)
	if valuation["notes"] != "only notes" {
		t.Fatalf("unexpected valuation payload: %#v", valuation)
	}
}

func TestFinancialCommandsRegistered(t *testing.T) {
	cmd := New()
	for _, args := range [][]string{
		{"balance-sheet", "show"},
		{"balances", "list"},
		{"family-settings", "show"},
		{"valuations", "create"},
	} {
		if _, _, err := cmd.Find(args); err != nil {
			t.Fatalf("expected command %v: %v", args, err)
		}
	}
}

func TestBuildValuationPayloads_RejectAmountsSureWouldMisread(t *testing.T) {
	// Rails casts the balance with String#to_d: "1,234.56" is stored as 1,
	// "abc" as 0, and a fifth decimal is rounded away by decimal(19,4).
	for amount, want := range map[string]string{
		"1,234.56": "amount must be a decimal number",
		"abc":      "amount must be a decimal number",
		"3.":       "amount must be a decimal number",
		"1.00001":  "amount has more than 4 decimal places",
	} {
		if _, err := buildValuationCreatePayload(valuationCreateOpts{AccountID: "a", Amount: amount, Date: "2026-05-01"}); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("create --amount %q: err = %v, want %q", amount, err, want)
		}
		if _, err := buildValuationUpdatePayload(valuationUpdateOpts{Amount: amount, Date: "2026-05-01"}); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("update --amount %q: err = %v, want %q", amount, err, want)
		}
	}
	// A liability's balance can be written negative.
	if _, err := buildValuationCreatePayload(valuationCreateOpts{AccountID: "a", Amount: "-250.5", Date: "2026-05-01"}); err != nil {
		t.Errorf("negative amount: %v", err)
	}
}
