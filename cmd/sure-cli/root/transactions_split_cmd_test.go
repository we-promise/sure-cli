package root

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestTransactionsSplitCommandsRegistered(t *testing.T) {
	root := New()
	for _, leaf := range []string{"show", "create", "update", "delete"} {
		got, _, err := root.Find([]string{"transactions", "split", leaf})
		if err != nil {
			t.Fatalf("transactions split %s not registered: %v", leaf, err)
		}
		if got.Name() != leaf {
			t.Fatalf("transactions split %s resolved to %q", leaf, got.Name())
		}
	}
}

func TestTransactionsSplitCommandShape(t *testing.T) {
	cmd := newTransactionsSplitCmd()
	for _, name := range []string{"create", "update"} {
		sub := findSub(t, cmd, name)
		for _, flag := range []string{"splits", "splits-file", "apply"} {
			if sub.Flags().Lookup(flag) == nil {
				t.Fatalf("expected flag %q on split %s", flag, name)
			}
		}
	}
	del := findSub(t, cmd, "delete")
	if del.Flags().Lookup("apply") == nil {
		t.Fatal("expected flag apply on split delete")
	}
	if del.Flags().Lookup("splits") != nil {
		t.Fatal("split delete takes no children")
	}
}

func TestTransactionSplitPath_EscapesID(t *testing.T) {
	if got := transactionSplitPath("a/b"); got != "/api/v1/transactions/a%2Fb/split" {
		t.Fatalf("path = %q", got)
	}
}

// Runs the real commands in dry-run, so a swapped method, path or body fails.
func TestTransactionsSplitCommands_DryRunRequests(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	format = "json"
	children := `[{"amount": "60.00", "name": "A"}, {"amount": "40.00"}]`
	wantBody := map[string]any{"split": map[string]any{"splits": []any{
		map[string]any{"amount": "60.00", "name": "A"},
		map[string]any{"amount": "40.00"},
	}}}

	cases := []struct {
		args       []string
		method     string
		expectBody bool
	}{
		{[]string{"create", "tx-1", "--splits", children}, "POST", true},
		{[]string{"update", "tx-1", "--splits", children}, "PATCH", true},
		{[]string{"delete", "tx-1"}, "DELETE", false},
	}
	for _, c := range cases {
		root := New()
		root.SetArgs(append([]string{"transactions", "split"}, c.args...))
		out := captureStdout(t, func() {
			if err := root.Execute(); err != nil {
				t.Fatalf("%v: %v", c.args, err)
			}
		})

		var env struct {
			Data struct {
				DryRun  bool `json:"dry_run"`
				Request struct {
					Method string         `json:"method"`
					Path   string         `json:"path"`
					Body   map[string]any `json:"body"`
				} `json:"request"`
			} `json:"data"`
		}
		if err := json.Unmarshal([]byte(out), &env); err != nil {
			t.Fatalf("%v: unmarshal: %v\nout=%q", c.args, err, out)
		}
		req := env.Data.Request
		if !env.Data.DryRun || req.Method != c.method || req.Path != "/api/v1/transactions/tx-1/split" {
			t.Errorf("%v: dry_run=%v method=%s path=%s", c.args, env.Data.DryRun, req.Method, req.Path)
		}
		if c.expectBody && !reflect.DeepEqual(req.Body, wantBody) {
			t.Errorf("%v: body = %#v", c.args, req.Body)
		}
		if !c.expectBody && req.Body != nil {
			t.Errorf("%v: unexpected body %#v", c.args, req.Body)
		}
	}
}
