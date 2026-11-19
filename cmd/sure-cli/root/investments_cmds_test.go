package root

import (
	"strings"
	"testing"
)

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

func validRecurringCreateOpts() recurringCreateOpts {
	return recurringCreateOpts{LastOccurrenceDate: "2026-04-01", NextExpectedDate: "2026-05-01"}
}

// Sure writes only the statuses it documents (WRITABLE_STATUSES).
func TestBuildRecurringPayloads_ValidateStatus(t *testing.T) {
	for _, status := range []string{"active", "inactive", ""} {
		o := validRecurringCreateOpts()
		o.Status = status
		if _, err := buildRecurringCreatePayload(o); err != nil {
			t.Fatalf("create status %q: unexpected err: %v", status, err)
		}
	}
	for _, status := range []string{"paused", "suggested", "ended", "Active"} {
		o := validRecurringCreateOpts()
		o.Status = status
		if _, err := buildRecurringCreatePayload(o); err == nil {
			t.Fatalf("create status %q: expected err", status)
		}
		if _, err := buildRecurringUpdatePayload(recurringUpdateOpts{Status: status}); err == nil {
			t.Fatalf("update status %q: expected err", status)
		}
	}
}

func TestBuildRecurringPayloads_BillFields(t *testing.T) {
	o := validRecurringCreateOpts()
	o.PaymentURL = "https://example.com/pay"
	o.Autopay = "true"
	o.Notes = "paper bill"
	create, err := buildRecurringCreatePayload(o)
	if err != nil {
		t.Fatalf("create: unexpected err: %v", err)
	}
	update, err := buildRecurringUpdatePayload(recurringUpdateOpts{PaymentURL: "https://example.com/pay", Autopay: "false", Notes: "paper bill"})
	if err != nil {
		t.Fatalf("update: unexpected err: %v", err)
	}
	for name, payload := range map[string]map[string]any{"create": create, "update": update} {
		r := payload["recurring_transaction"].(map[string]any)
		if r["payment_url"] != "https://example.com/pay" || r["notes"] != "paper bill" {
			t.Fatalf("%s: unexpected payload %#v", name, r)
		}
	}
	if got := create["recurring_transaction"].(map[string]any)["autopay"]; got != true {
		t.Fatalf("create autopay = %#v, want true", got)
	}
	if got := update["recurring_transaction"].(map[string]any)["autopay"]; got != false {
		t.Fatalf("update autopay = %#v, want false", got)
	}
}

func TestBuildRecurringPayloads_RejectInvalidAutopay(t *testing.T) {
	o := validRecurringCreateOpts()
	o.Autopay = "yes"
	if _, err := buildRecurringCreatePayload(o); err == nil {
		t.Fatal("create: expected err for --autopay yes")
	}
	if _, err := buildRecurringUpdatePayload(recurringUpdateOpts{Autopay: "1"}); err == nil {
		t.Fatal("update: expected err for --autopay 1")
	}
}

func TestRecurringTransactions_Flags(t *testing.T) {
	group := findSub(t, New(), "recurring-transactions")
	for _, sub := range []string{"create", "update"} {
		cmd := findSub(t, group, sub)
		for _, name := range []string{"status", "payment-url", "autopay", "notes"} {
			if cmd.Flags().Lookup(name) == nil {
				t.Fatalf("%s: expected --%s flag", sub, name)
			}
		}
		if usage := cmd.Flags().Lookup("status").Usage; !strings.Contains(usage, "active|inactive") {
			t.Fatalf("%s: --status help should name active|inactive, got %q", sub, usage)
		}
	}
}

func TestInvestmentCommandsRegistered(t *testing.T) {
	cmd := New()
	for _, args := range [][]string{
		{"holdings", "show"},
		{"securities", "list"},
		{"security-prices", "show"},
		{"security-prices", "create"},
		{"trades", "create"},
		{"recurring-transactions", "delete"},
	} {
		if _, _, err := cmd.Find(args); err != nil {
			t.Fatalf("expected command %v: %v", args, err)
		}
	}
}
