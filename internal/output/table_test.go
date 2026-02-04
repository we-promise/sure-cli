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
