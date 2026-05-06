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
	tag := payload["tag"].(map[string]any)
	if tag["name"] != "Travel" || tag["color"] != "#3b82f6" {
		t.Fatalf("unexpected tag payload: %#v", tag)
	}
}

func TestBuildValuationCreatePayload_Upsert(t *testing.T) {
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

func TestBuildValuationUpdatePayload_RequiresAmountAndDateTogether(t *testing.T) {
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

func TestBuildTradeCreatePayload_RequiresSecurityIdentifier(t *testing.T) {
	_, err := buildTradeCreatePayload(tradeCreateOpts{
		AccountID: "acc_123",
		Date:      "2026-05-01",
		Type:      "buy",
		Qty:       "1",
		Price:     "10",
	})
	if err == nil {
		t.Fatal("expected missing security identifier error")
	}
}

func TestBuildTradeUpdatePayload(t *testing.T) {
	payload, err := buildTradeUpdatePayload(tradeUpdateOpts{Qty: "2", Price: "11.50", Type: "sell"})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	trade := payload["trade"].(map[string]any)
	if trade["qty"] != "2" || trade["price"] != "11.50" || trade["type"] != "sell" {
		t.Fatalf("unexpected trade payload: %#v", trade)
	}
}

func TestBuildRecurringCreatePayload_RequiresDates(t *testing.T) {
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
	recurring := payload["recurring_transaction"].(map[string]any)
	if recurring["name"] != "Rent" || recurring["next_expected_date"] != "2026-05-01" {
		t.Fatalf("unexpected recurring payload: %#v", recurring)
	}
}

func TestUsersResetCommandShape(t *testing.T) {
	cmd := newUsersCmd()
	reset, _, err := cmd.Find([]string{"reset"})
	if err != nil {
		t.Fatalf("find reset command: %v", err)
	}
	if reset.Flags().Lookup("apply") == nil {
		t.Fatal("expected reset --apply flag")
	}
	if _, _, err := cmd.Find([]string{"reset", "status"}); err != nil {
		t.Fatalf("find reset status command: %v", err)
	}
}
