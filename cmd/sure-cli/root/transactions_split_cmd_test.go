package root

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
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

func TestTxSplitPath_EscapesID(t *testing.T) {
	if got := txSplitPath("a/b"); got != "/api/v1/transactions/a%2Fb/split" {
		t.Fatalf("path = %q", got)
	}
}

func splitsOf(t *testing.T, body map[string]any) []map[string]any {
	t.Helper()
	return body["split"].(map[string]any)["splits"].([]map[string]any)
}

func TestBuildTxSplitPayload_Array(t *testing.T) {
	body, err := buildTxSplitPayload([]byte(`[
		{"amount": "60.00", "name": "Groceries", "category_id": "cat-1", "excluded": true},
		{"amount": "40.00"}
	]`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []map[string]any{
		{"amount": "60.00", "name": "Groceries", "category_id": "cat-1", "excluded": true},
		{"amount": "40.00"},
	}
	if got := splitsOf(t, body); !reflect.DeepEqual(got, want) {
		t.Fatalf("splits = %#v, want %#v", got, want)
	}
}

// The server stores four decimals and needs an exact sum, so the text the
// caller wrote must reach it unchanged, whether quoted or a bare number.
func TestBuildTxSplitPayload_KeepsAmountText(t *testing.T) {
	body, err := buildTxSplitPayload([]byte(`[{"amount": "50.0049"}, {"amount": 49.9951}, {"amount": -0.10}]`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var got []string
	for _, s := range splitsOf(t, body) {
		got = append(got, s["amount"].(string))
	}
	if want := []string{"50.0049", "49.9951", "-0.10"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("amounts = %v, want %v", got, want)
	}
}

const txSplitShowOutput = `{
  "data": {
    "transaction_id": "parent-1",
    "amount": "$100.00",
    "amount_decimal": "100.0",
    "children": [
      {"id": "c1", "amount": "$50.00", "amount_decimal": "50.0049", "amount_cents": 5000,
       "name": "Excluded", "excluded": true, "category": {"id": "cat-1", "name": "Food"},
       "account": {"id": "a1"}, "tags": [], "merchant": null},
      {"id": "c2", "amount": "$50.00", "amount_decimal": "49.9951",
       "name": "Counted", "excluded": false, "category": null}
    ]
  },
  "meta": {"status": 200}
}`

func TestBuildTxSplitPayload_AcceptsShowOutput(t *testing.T) {
	want := []map[string]any{
		{"amount": "50.0049", "name": "Excluded", "category_id": "cat-1", "excluded": true},
		{"amount": "49.9951", "name": "Counted", "excluded": false},
	}

	body, err := buildTxSplitPayload([]byte(txSplitShowOutput))
	if err != nil {
		t.Fatalf("envelope: unexpected error: %v", err)
	}
	if got := splitsOf(t, body); !reflect.DeepEqual(got, want) {
		t.Fatalf("envelope: splits = %#v, want %#v", got, want)
	}

	var env struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal([]byte(txSplitShowOutput), &env); err != nil {
		t.Fatal(err)
	}
	body, err = buildTxSplitPayload(env.Data)
	if err != nil {
		t.Fatalf("data only: unexpected error: %v", err)
	}
	if got := splitsOf(t, body); !reflect.DeepEqual(got, want) {
		t.Fatalf("data only: splits = %#v, want %#v", got, want)
	}
}

func TestBuildTxSplitPayload_Rejects(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{"invalid json", `[{"amount":`, "not valid JSON"},
		{"empty array", `[]`, "non-empty JSON array"},
		{"not an array", `"x"`, "non-empty JSON array"},
		{"object without children", `{"amount": "1"}`, "non-empty JSON array"},
		{"element not an object", `[true]`, "split 1 must be an object"},
		{"missing amount", `[{"amount": "1"}, {"name": "x"}]`, "split 2: amount is required"},
		{"display amount", `[{"amount": "$60.00"}]`, "plain decimal"},
		{"exponent amount", `[{"amount": 1e2}]`, "plain decimal"},
		{"null amount", `[{"amount": null}]`, "plain decimal"},
		{"unknown field", `[{"amount": "1", "catgory_id": "x"}]`, `unknown field "catgory_id"`},
		{"name not a string", `[{"amount": "1", "name": 5}]`, "name must be a string"},
		{"category not a string", `[{"amount": "1", "category_id": 5}]`, "category_id must be a string"},
		{"excluded not a bool", `[{"amount": "1", "excluded": "yes"}]`, "excluded must be true or false"},
		{"hand-written notes", `[{"amount": "1", "notes": "x"}]`, `unknown field "notes"`},
	}
	for _, c := range cases {
		_, err := buildTxSplitPayload([]byte(c.in))
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: err = %v, want it to contain %q", c.name, err, c.want)
		}
	}
}

func TestReadTxSplitInput(t *testing.T) {
	file := filepath.Join(t.TempDir(), "children.json")
	if err := os.WriteFile(file, []byte("from-file"), 0o600); err != nil {
		t.Fatal(err)
	}
	stdin := strings.NewReader("from-stdin")

	cases := []struct {
		name string
		opts txSplitOpts
		want string
	}{
		{"inline", txSplitOpts{Splits: "inline"}, "inline"},
		{"file", txSplitOpts{SplitsFile: file}, "from-file"},
		{"stdin", txSplitOpts{SplitsFile: "-"}, "from-stdin"},
	}
	for _, c := range cases {
		got, err := readTxSplitInput(c.opts, stdin)
		if err != nil || string(got) != c.want {
			t.Errorf("%s: got %q, %v; want %q", c.name, got, err, c.want)
		}
	}

	if _, err := readTxSplitInput(txSplitOpts{}, stdin); err == nil {
		t.Error("expected an error when neither source is given")
	}
	if _, err := readTxSplitInput(txSplitOpts{Splits: "x", SplitsFile: file}, stdin); err == nil {
		t.Error("expected an error when both sources are given")
	}
	if _, err := readTxSplitInput(txSplitOpts{SplitsFile: filepath.Join(t.TempDir(), "missing.json")}, stdin); err == nil {
		t.Error("expected an error for a missing file")
	}
}

// An edited "amount" next to an untouched amount_decimal must not be dropped
// in favour of the stale exact value.
func TestBuildTxSplitPayload_AmountAndDecimal(t *testing.T) {
	cases := []struct {
		name, child, want, wantErr string
	}{
		{"display amount uses decimal", `{"id": "c1", "amount": "$60.00", "amount_decimal": "60.0049"}`, "60.0049", ""},
		{"plain amount agreeing", `{"id": "c1", "amount": "60.00", "amount_decimal": "60.0"}`, "60.0", ""},
		{"edited amount", `{"id": "c1", "amount": "50.00", "amount_decimal": "60.0"}`, "", "split 1: amount (50.00) and amount_decimal (60.0) disagree"},
		{"decimal removed", `{"id": "c1", "amount": "50.00"}`, "50.00", ""},
		{"edited decimal", `{"id": "c1", "amount": "$60.00", "amount_decimal": "50.0"}`, "50.0", ""},
	}
	for _, c := range cases {
		body, err := buildTxSplitPayload([]byte("[" + c.child + "]"))
		if c.wantErr != "" {
			if err == nil || !strings.Contains(err.Error(), c.wantErr) {
				t.Errorf("%s: err = %v, want %q", c.name, err, c.wantErr)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: unexpected error: %v", c.name, err)
			continue
		}
		if got := splitsOf(t, body)[0]["amount"]; got != c.want {
			t.Errorf("%s: amount = %v, want %s", c.name, got, c.want)
		}
	}
}

func TestBuildTxSplitPayload_RejectsChildrenThatWouldLoseData(t *testing.T) {
	cases := map[string]string{
		`{"id": "c1", "amount_decimal": "1", "tags": [{"id": "t1"}]}`:                  "has tags, which",
		`{"id": "c1", "amount_decimal": "1", "notes": "receipt"}`:                      "has notes, which",
		`{"id": "c1", "amount_decimal": "1", "notes": "receipt", "tags": [{"id": 1}]}`: "has tags and notes",
	}
	for child, want := range cases {
		_, err := buildTxSplitPayload([]byte("[" + child + "]"))
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: err = %v, want %q", child, err, want)
		}
	}

	// Empty tags and notes, as show prints them for most children, are fine.
	if _, err := buildTxSplitPayload([]byte(`[{"id": "c1", "amount_decimal": "1", "tags": [], "notes": null}]`)); err != nil {
		t.Errorf("empty tags/notes: unexpected error: %v", err)
	}
}

func TestBuildTxSplitPayload_NullCategoryClears(t *testing.T) {
	body, err := buildTxSplitPayload([]byte(`[{"id": "c1", "amount_decimal": "1", "category": {"id": "cat-1"}, "category_id": null}]`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, has := splitsOf(t, body)[0]["category_id"]; has {
		t.Fatalf("category_id = %v, want it cleared", got)
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
