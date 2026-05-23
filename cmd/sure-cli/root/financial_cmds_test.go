package root

import "testing"

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
