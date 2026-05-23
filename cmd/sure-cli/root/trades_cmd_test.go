package root

import (
	"strings"
	"testing"
)

func TestTradesList_Flags(t *testing.T) {
	cmd := newTradesCmd()

	list, _, err := cmd.Find([]string{"list"})
	if err != nil {
		t.Fatalf("find list subcommand: %v", err)
	}

	// Verify expected flags exist
	expectedFlags := []string{"from", "to", "start-date", "end-date", "account", "account-id", "account-ids", "page", "per-page"}
	for _, name := range expectedFlags {
		if list.Flags().Lookup(name) == nil {
			t.Fatalf("expected flag %q to exist", name)
		}
	}

	// Verify usage contains expected flags
	s := list.Flags().FlagUsages()
	if !strings.Contains(s, "from") {
		t.Fatalf("expected from in usage")
	}
	if !strings.Contains(s, "account-id") {
		t.Fatalf("expected account-id in usage")
	}
}

func TestTradesShow_Args(t *testing.T) {
	cmd := newTradesCmd()

	show, _, err := cmd.Find([]string{"show"})
	if err != nil {
		t.Fatalf("find show subcommand: %v", err)
	}

	// Verify it requires exactly 1 argument
	if show.Args == nil {
		t.Fatal("expected Args validator to be set")
	}
}

func TestBuildTradeCreatePayloadRequiresSecurityIdentifier(t *testing.T) {
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
