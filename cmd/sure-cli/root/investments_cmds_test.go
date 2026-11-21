package root

import (
	"encoding/json"
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
		Currency:           "EUR",
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
	return recurringCreateOpts{Amount: "1200", Currency: "EUR", ExpectedDayOfMonth: "1", LastOccurrenceDate: "2026-04-01", NextExpectedDate: "2026-05-01"}
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

func TestBuildRecurringCreatePayload_RequiresWhatSureValidates(t *testing.T) {
	// RecurringTransaction validates amount, currency and expected_day_of_month
	// (1-31) and the controller fills none of them in.
	full := func(edit func(*recurringCreateOpts)) recurringCreateOpts {
		o := recurringCreateOpts{Name: "Rent", Amount: "1200", Currency: "EUR", ExpectedDayOfMonth: "1", LastOccurrenceDate: "2026-04-01", NextExpectedDate: "2026-05-01"}
		edit(&o)
		return o
	}
	for name, tc := range map[string]struct {
		opts recurringCreateOpts
		want string
	}{
		"no amount":           {full(func(o *recurringCreateOpts) { o.Amount = "" }), "--amount is required"},
		"no currency":         {full(func(o *recurringCreateOpts) { o.Currency = "" }), "--currency is required"},
		"no day of month":     {full(func(o *recurringCreateOpts) { o.ExpectedDayOfMonth = "" }), "--expected-day-of-month is required"},
		"day of month 0":      {full(func(o *recurringCreateOpts) { o.ExpectedDayOfMonth = "0" }), "--expected-day-of-month must be a whole number from 1 to 31"},
		"day of month 32":     {full(func(o *recurringCreateOpts) { o.ExpectedDayOfMonth = "32" }), "--expected-day-of-month must be a whole number from 1 to 31"},
		"amount with comma":   {full(func(o *recurringCreateOpts) { o.Amount = "1,234.5" }), "amount must be a decimal number"},
		"min with 5 decimals": {full(func(o *recurringCreateOpts) { o.ExpectedAmountMin = "1.00001" }), "expected-amount-min has more than 4 decimal places"},
		"max not a number":    {full(func(o *recurringCreateOpts) { o.ExpectedAmountMax = "lots" }), "expected-amount-max must be a decimal number"},
		"avg trailing point":  {full(func(o *recurringCreateOpts) { o.ExpectedAmountAvg = "3." }), "expected-amount-avg must be a decimal number"},
		"US-style last date":  {full(func(o *recurringCreateOpts) { o.LastOccurrenceDate = "04/01/2026" }), "invalid date"},
		"bad next date":       {full(func(o *recurringCreateOpts) { o.NextExpectedDate = "2026-02-30" }), "invalid date"},
	} {
		if _, err := buildRecurringCreatePayload(tc.opts); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: err = %v, want %q", name, err, tc.want)
		}
	}
	if _, err := buildRecurringCreatePayload(full(func(o *recurringCreateOpts) { o.Amount = "-2500" })); err != nil {
		t.Errorf("income series (negative amount): %v", err)
	}
}

func TestBuildRecurringUpdatePayload_ChecksDayAndDate(t *testing.T) {
	for opts, want := range map[recurringUpdateOpts]string{
		{ExpectedDayOfMonth: "40"}:       "--expected-day-of-month must be a whole number from 1 to 31",
		{NextExpectedDate: "05/01/2026"}: "invalid date",
	} {
		if _, err := buildRecurringUpdatePayload(opts); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%+v: err = %v, want %q", opts, err, want)
		}
	}
}

func TestRecurringUpdate_EmptyNotesAndPaymentURLClearThem(t *testing.T) {
	// Sure stores notes as given and normalizes a blank payment_url to nil,
	// so an explicit empty flag is how a user clears either. A flag left out
	// is not sent.
	body := func(args ...string) map[string]any {
		t.Helper()
		cmd := newRecurringTransactionsCmd()
		cmd.SetArgs(append([]string{"update", "r1"}, args...))
		out := captureStdout(t, func() { _ = cmd.Execute() })
		var env struct {
			Data struct {
				Request struct {
					Body map[string]map[string]any `json:"body"`
				} `json:"request"`
			} `json:"data"`
		}
		if err := json.Unmarshal([]byte(out), &env); err != nil {
			t.Fatalf("decode %s: %v", out, err)
		}
		return env.Data.Request.Body["recurring_transaction"]
	}
	cleared := body("--notes", "", "--payment-url", "")
	if v, ok := cleared["notes"]; !ok || v != "" {
		t.Errorf("--notes \"\" sent notes=%#v (present=%v), want \"\"", v, ok)
	}
	if v, ok := cleared["payment_url"]; !ok || v != "" {
		t.Errorf("--payment-url \"\" sent payment_url=%#v (present=%v), want \"\"", v, ok)
	}
	kept := body("--status", "active")
	if _, ok := kept["notes"]; ok {
		t.Errorf("notes sent without --notes: %#v", kept)
	}
	if _, ok := kept["payment_url"]; ok {
		t.Errorf("payment_url sent without --payment-url: %#v", kept)
	}
}
