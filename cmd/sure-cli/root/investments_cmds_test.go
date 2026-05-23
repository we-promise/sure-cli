package root

import "testing"

func TestBuildRecurringCreatePayloadRequiresDates(t *testing.T) {
	if _, err := buildRecurringCreatePayload(recurringCreateOpts{}); err == nil {
		t.Fatal("expected missing dates error")
	}
	payload, err := buildRecurringCreatePayload(recurringCreateOpts{
		Name:               "Rent",
		Amount:             "1200",
		LastOccurrenceDate: "2026-04-01",
		NextExpectedDate:   "2026-05-01",
		ExpectedDayOfMonth: "1",
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	recurring, ok := payload["recurring_transaction"].(map[string]any)
	if !ok || recurring == nil {
		t.Fatalf("expected recurring_transaction payload, got %#v", payload["recurring_transaction"])
	}
	if recurring["name"] != "Rent" || recurring["next_expected_date"] != "2026-05-01" {
		t.Fatalf("unexpected recurring payload: %#v", recurring)
	}
}

func TestInvestmentCommandsRegistered(t *testing.T) {
	cmd := New()
	for _, args := range [][]string{
		{"holdings", "show"},
		{"securities", "list"},
		{"security-prices", "show"},
		{"trades", "create"},
		{"recurring-transactions", "delete"},
	} {
		if _, _, err := cmd.Find(args); err != nil {
			t.Fatalf("expected command %v: %v", args, err)
		}
	}
}
