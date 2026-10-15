package root

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func splitsOf(t *testing.T, body map[string]any) []map[string]any {
	t.Helper()
	return body["split"].(map[string]any)["splits"].([]map[string]any)
}

func TestBuildTransactionSplitPayload_Array(t *testing.T) {
	body, err := buildTransactionSplitPayload([]byte(`[
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
func TestBuildTransactionSplitPayload_KeepsAmountText(t *testing.T) {
	body, err := buildTransactionSplitPayload([]byte(`[{"amount": "50.0049"}, {"amount": 49.9951}, {"amount": -0.10}]`))
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

const transactionSplitShowOutput = `{
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

func TestBuildTransactionSplitPayload_AcceptsShowOutput(t *testing.T) {
	want := []map[string]any{
		{"amount": "50.0049", "name": "Excluded", "category_id": "cat-1", "excluded": true},
		{"amount": "49.9951", "name": "Counted", "excluded": false},
	}

	body, err := buildTransactionSplitPayload([]byte(transactionSplitShowOutput))
	if err != nil {
		t.Fatalf("envelope: unexpected error: %v", err)
	}
	if got := splitsOf(t, body); !reflect.DeepEqual(got, want) {
		t.Fatalf("envelope: splits = %#v, want %#v", got, want)
	}

	var env struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal([]byte(transactionSplitShowOutput), &env); err != nil {
		t.Fatal(err)
	}
	body, err = buildTransactionSplitPayload(env.Data)
	if err != nil {
		t.Fatalf("data only: unexpected error: %v", err)
	}
	if got := splitsOf(t, body); !reflect.DeepEqual(got, want) {
		t.Fatalf("data only: splits = %#v, want %#v", got, want)
	}
}

func TestBuildTransactionSplitPayload_Rejects(t *testing.T) {
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
		_, err := buildTransactionSplitPayload([]byte(c.in))
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: err = %v, want it to contain %q", c.name, err, c.want)
		}
	}
}

func TestReadTransactionSplitInput(t *testing.T) {
	file := filepath.Join(t.TempDir(), "children.json")
	if err := os.WriteFile(file, []byte("from-file"), 0o600); err != nil {
		t.Fatal(err)
	}
	stdin := strings.NewReader("from-stdin")

	cases := []struct {
		name string
		opts transactionSplitOpts
		want string
	}{
		{"inline", transactionSplitOpts{Splits: "inline"}, "inline"},
		{"file", transactionSplitOpts{SplitsFile: file}, "from-file"},
		{"stdin", transactionSplitOpts{SplitsFile: "-"}, "from-stdin"},
	}
	for _, c := range cases {
		got, err := readTransactionSplitInput(c.opts, stdin)
		if err != nil || string(got) != c.want {
			t.Errorf("%s: got %q, %v; want %q", c.name, got, err, c.want)
		}
	}

	if _, err := readTransactionSplitInput(transactionSplitOpts{}, stdin); err == nil {
		t.Error("expected an error when neither source is given")
	}
	if _, err := readTransactionSplitInput(transactionSplitOpts{Splits: "x", SplitsFile: file}, stdin); err == nil {
		t.Error("expected an error when both sources are given")
	}
	if _, err := readTransactionSplitInput(transactionSplitOpts{SplitsFile: filepath.Join(t.TempDir(), "missing.json")}, stdin); err == nil {
		t.Error("expected an error for a missing file")
	}
}

// An edited "amount" next to an untouched amount_decimal must not be dropped
// in favour of the stale exact value.
func TestTransactionSplitChildAmount(t *testing.T) {
	cases := []struct {
		name, child, want, wantErr string
	}{
		{"display amount uses decimal", `{"amount": "$60.00", "amount_decimal": "60.0049"}`, "60.0049", ""},
		{"plain amount agreeing", `{"amount": "60.00", "amount_decimal": "60.0"}`, "60.0", ""},
		{"edited amount", `{"amount": "50.00", "amount_decimal": "60.0"}`, "", "amount (50.00) and amount_decimal (60.0) disagree"},
		{"edited decimal", `{"amount": "$60.00", "amount_decimal": "50.0"}`, "50.0", ""},
		{"decimal removed", `{"amount": "50.00"}`, "50.00", ""},
		{"decimal only", `{"amount_decimal": "50.0"}`, "50.0", ""},
		{"bare number", `{"amount": 49.9951}`, "49.9951", ""},
		{"neither", `{"name": "x"}`, "", "amount is required"},
		{"display only", `{"amount": "$60.00"}`, "", "plain decimal"},
	}
	for _, c := range cases {
		got, err := transactionSplitChildAmount(decodeChild(t, c.child))
		if c.wantErr != "" {
			if err == nil || !strings.Contains(err.Error(), c.wantErr) {
				t.Errorf("%s: err = %v, want %q", c.name, err, c.wantErr)
			}
			continue
		}
		if err != nil || got != c.want {
			t.Errorf("%s: got %q, %v; want %q", c.name, got, err, c.want)
		}
	}
}

func TestTransactionSplitChildCategoryID(t *testing.T) {
	cases := []struct {
		name, child, want, wantErr string
	}{
		{"explicit", `{"category_id": "cat-2"}`, "cat-2", ""},
		{"copied from show", `{"category": {"id": "cat-1"}}`, "cat-1", ""},
		{"explicit wins over copied", `{"category_id": "cat-2", "category": {"id": "cat-1"}}`, "cat-2", ""},
		{"null clears copied", `{"category_id": null, "category": {"id": "cat-1"}}`, "", ""},
		{"show with no category", `{"category": null}`, "", ""},
		{"absent", `{}`, "", ""},
		{"not a string", `{"category_id": 5}`, "", "category_id must be a string"},
	}
	for _, c := range cases {
		got, err := transactionSplitChildCategoryID(decodeChild(t, c.child))
		if c.wantErr != "" {
			if err == nil || !strings.Contains(err.Error(), c.wantErr) {
				t.Errorf("%s: err = %v, want %q", c.name, err, c.wantErr)
			}
			continue
		}
		if err != nil || got != c.want {
			t.Errorf("%s: got %q, %v; want %q", c.name, got, err, c.want)
		}
	}
}

func decodeChild(t *testing.T, raw string) map[string]any {
	t.Helper()
	dec := json.NewDecoder(strings.NewReader(raw))
	dec.UseNumber()
	var child map[string]any
	if err := dec.Decode(&child); err != nil {
		t.Fatalf("decode %s: %v", raw, err)
	}
	return child
}

func TestBuildTransactionSplitPayload_RejectsChildrenThatWouldLoseData(t *testing.T) {
	cases := map[string]string{
		`{"id": "c1", "amount_decimal": "1", "tags": [{"id": "t1"}]}`:                  "has tags, which",
		`{"id": "c1", "amount_decimal": "1", "notes": "receipt"}`:                      "has notes, which",
		`{"id": "c1", "amount_decimal": "1", "notes": "receipt", "tags": [{"id": 1}]}`: "has tags and notes",
	}
	for child, want := range cases {
		_, err := buildTransactionSplitPayload([]byte("[" + child + "]"))
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: err = %v, want %q", child, err, want)
		}
	}

	// Empty tags and notes, as show prints them for most children, are fine.
	if _, err := buildTransactionSplitPayload([]byte(`[{"id": "c1", "amount_decimal": "1", "tags": [], "notes": null}]`)); err != nil {
		t.Errorf("empty tags/notes: unexpected error: %v", err)
	}
}

func TestBuildTransactionSplitPayload_NullCategoryClears(t *testing.T) {
	body, err := buildTransactionSplitPayload([]byte(`[{"id": "c1", "amount_decimal": "1", "category": {"id": "cat-1"}, "category_id": null}]`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, has := splitsOf(t, body)[0]["category_id"]; has {
		t.Fatalf("category_id = %v, want it cleared", got)
	}
}
