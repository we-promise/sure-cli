package output

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	fn()

	_ = w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	return buf.String()
}

func TestPrintTable_Accounts(t *testing.T) {
	env := Envelope{Data: map[string]any{"accounts": []any{map[string]any{"id": "1", "name": "A", "account_type": "depository", "currency": "EUR", "balance": "€1.00", "classification": "asset"}}}}
	out := captureStdout(t, func() {
		ok := PrintTable(env)
		if !ok {
			t.Fatalf("expected ok")
		}
	})
	if !strings.Contains(out, "depository") || !strings.Contains(out, "€1.00") {
		t.Fatalf("unexpected table output: %q", out)
	}
}

func TestPrintTable_Transactions(t *testing.T) {
	env := Envelope{Data: map[string]any{"transactions": []any{map[string]any{"id": "t1", "date": "2026-01-01", "name": "coffee", "classification": "expense", "amount": "€2.00", "account": map[string]any{"name": "main"}}}}}
	out := captureStdout(t, func() {
		ok := PrintTable(env)
		if !ok {
			t.Fatalf("expected ok")
		}
	})
	if !strings.Contains(out, "coffee") || !strings.Contains(out, "main") {
		t.Fatalf("unexpected table output: %q", out)
	}
}

// Shapes follow Sure's api/v1/holdings/_holding and api/v1/trades/_trade views.
func TestPrintTable_Holdings(t *testing.T) {
	env := Envelope{Data: map[string]any{"holdings": []any{
		map[string]any{
			"id": "h1", "date": "2026-05-01", "qty": "10.0", "price": "$12.34", "amount": "$123.40", "avg_cost": "$10.00",
			"security": map[string]any{"ticker": "AAPL", "name": "Apple"},
			"account":  map[string]any{"name": "Brokerage"},
			"lots":     []any{map[string]any{"id": "l1"}, map[string]any{"id": "l2"}},
		},
		map[string]any{
			"id": "h2", "date": "2026-04-30", "qty": "10.0", "price": "$12.00", "amount": "$120.00", "avg_cost": nil,
			"security": map[string]any{"ticker": "AAPL", "name": "Apple"},
			"account":  map[string]any{"name": "Brokerage"},
			"lots":     nil,
		},
	}}}
	out := captureStdout(t, func() {
		if !PrintTable(env) {
			t.Fatalf("expected ok")
		}
	})
	for _, want := range []string{"AAPL", "Apple", "10.0", "$12.34", "$123.40", "$10.00", "Brokerage", "LOTS", "2026-04-30"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in table output: %q", want, out)
		}
	}
	if strings.Contains(out, "<nil>") {
		t.Fatalf("table must not print <nil>: %q", out)
	}
}

func TestPrintTable_Trades(t *testing.T) {
	env := Envelope{Data: map[string]any{"trades": []any{
		map[string]any{
			"id": "t1", "date": "2026-05-01", "qty": "-2.0", "price": "$101.25", "amount": "-$202.50",
			"investment_activity_label": "Sell", "lot_source_trade_id": "t0",
			"security": map[string]any{"ticker": "AAPL"},
			"account":  map[string]any{"name": "Brokerage"},
		},
		map[string]any{
			"id": "t2", "date": "2026-05-02", "qty": "0.0", "price": "$0.00", "amount": "-$3.10",
			"investment_activity_label": "Interest", "lot_source_trade_id": nil,
			"security": nil,
			"account":  map[string]any{"name": "Brokerage"},
		},
	}}}
	out := captureStdout(t, func() {
		if !PrintTable(env) {
			t.Fatalf("expected ok")
		}
	})
	for _, want := range []string{"AAPL", "Sell", "-2.0", "$101.25", "-$202.50", "LOT_SOURCE", "t0", "Interest", "Brokerage"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in table output: %q", want, out)
		}
	}
	if strings.Contains(out, "<nil>") {
		t.Fatalf("table must not print <nil>: %q", out)
	}
}
