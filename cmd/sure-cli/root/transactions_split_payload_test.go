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

func TestBuildTransactionSplitPayload_RejectsNotes(t *testing.T) {
	_, err := buildTransactionSplitPayload([]byte(`[{"id": "c1", "amount_decimal": "1", "notes": "receipt", "tags": [{"id": "t1"}]}]`))
	if err == nil || !strings.Contains(err.Error(), "has notes, which") {
		t.Errorf("err = %v, want it to refuse notes", err)
	}

	// Empty tags and notes, as show prints them for most children, are fine.
	if _, err := buildTransactionSplitPayload([]byte(`[{"id": "c1", "amount_decimal": "1", "tags": [], "notes": null}]`)); err != nil {
		t.Errorf("empty tags/notes: unexpected error: %v", err)
	}
}

func TestBuildTransactionSplitPayload_ForwardsTagsAndMerchant(t *testing.T) {
	cases := []struct {
		name, child string
		want        map[string]any
	}{
		{"hand-written ids", `{"amount": "1", "merchant_id": "m1", "tag_ids": ["t1", "t2"]}`,
			map[string]any{"amount": "1", "merchant_id": "m1", "tag_ids": []string{"t1", "t2"}}},
		{"copied from show", `{"id": "c1", "amount_decimal": "1", "merchant": {"id": "m1", "name": "Shop"}, "tags": [{"id": "t1", "name": "Trip", "color": "#fff"}]}`,
			map[string]any{"amount": "1", "merchant_id": "m1", "tag_ids": []string{"t1"}}},
		{"explicit ids win over copied", `{"id": "c1", "amount_decimal": "1", "merchant": {"id": "m1"}, "merchant_id": "m2", "tags": [{"id": "t1"}], "tag_ids": ["t2"]}`,
			map[string]any{"amount": "1", "merchant_id": "m2", "tag_ids": []string{"t2"}}},
		{"empty tag_ids clears copied tags", `{"id": "c1", "amount_decimal": "1", "tags": [{"id": "t1"}], "tag_ids": []}`,
			map[string]any{"amount": "1"}},
		{"null tag_ids clears copied tags", `{"id": "c1", "amount_decimal": "1", "tags": [{"id": "t1"}], "tag_ids": null}`,
			map[string]any{"amount": "1"}},
		{"show with neither", `{"id": "c1", "amount_decimal": "1", "merchant": null, "tags": []}`,
			map[string]any{"amount": "1"}},
	}
	for _, c := range cases {
		body, err := buildTransactionSplitPayload([]byte("[" + c.child + "]"))
		if err != nil {
			t.Errorf("%s: unexpected error: %v", c.name, err)
			continue
		}
		if got := splitsOf(t, body)[0]; !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: split = %#v, want %#v", c.name, got, c.want)
		}
	}
}

func TestBuildTransactionSplitPayload_RejectsBadTagsAndMerchant(t *testing.T) {
	cases := []struct {
		name, child, want string
	}{
		{"merchant_id not a string", `{"amount": "1", "merchant_id": 5}`, "merchant_id must be a string"},
		{"null merchant_id", `{"amount": "1", "merchant_id": null}`, "merchant_id cannot be null"},
		{"tag_ids not an array", `{"amount": "1", "tag_ids": "t1"}`, "tag_ids must be an array of tag ids"},
		{"tag_ids with a non-string", `{"amount": "1", "tag_ids": ["t1", 2]}`, "tag_ids must be an array of tag ids"},
		{"tags as plain ids", `{"amount": "1", "tags": ["t1"]}`, "use tag_ids"},
		{"tag without an id", `{"id": "c1", "amount_decimal": "1", "tags": [{"name": "Trip"}]}`, "use tag_ids"},
	}
	for _, c := range cases {
		_, err := buildTransactionSplitPayload([]byte("[" + c.child + "]"))
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: err = %v, want it to contain %q", c.name, err, c.want)
		}
	}
}

// Every field the transaction partial prints, so the round trip keeps working
// while a typo on a copied child is still caught.
const transactionSplitFullShowChild = `{
  "id": "c1", "entry_id": "e1", "date": "2026-02-04", "amount": "$60.00",
  "amount_cents": 6000, "signed_amount_cents": -6000, "amount_decimal": "60.0",
  "currency": "USD", "name": "Groceries", "notes": null, "external_id": null,
  "source": null, "user_modified": true, "classification": "expense",
  "kind": "standard", "excluded": false, "split_parent": false,
  "account": {"id": "a1", "name": "Checking", "account_type": "depository"},
  "category": {"id": "cat-1", "name": "Food", "color": "#fff", "icon": "utensils"},
  "merchant": {"id": "m1", "name": "Shop"},
  "tags": [{"id": "t1", "name": "Trip", "color": "#fff"}],
  "transfer": null,
  "created_at": "2026-02-04T00:00:00Z", "updated_at": "2026-02-04T00:00:00Z"
}`

func TestBuildTransactionSplitPayload_ChecksKeysOnShowChildren(t *testing.T) {
	body, err := buildTransactionSplitPayload([]byte("[" + transactionSplitFullShowChild + "]"))
	if err != nil {
		t.Fatalf("full show child: unexpected error: %v", err)
	}
	want := map[string]any{
		"amount": "60.0", "name": "Groceries", "category_id": "cat-1",
		"merchant_id": "m1", "tag_ids": []string{"t1"}, "excluded": false,
	}
	if got := splitsOf(t, body)[0]; !reflect.DeepEqual(got, want) {
		t.Fatalf("full show child: split = %#v, want %#v", got, want)
	}

	_, err = buildTransactionSplitPayload([]byte(`[{"id": "c1", "amount_decimal": "1", "merchnt_id": "m1"}]`))
	if err == nil || !strings.Contains(err.Error(), `unknown field "merchnt_id"`) {
		t.Errorf("typo on a show child: err = %v", err)
	}

	// The fields show adds stay off hand-written children.
	_, err = buildTransactionSplitPayload([]byte(`[{"amount": "1", "date": "2026-02-04"}]`))
	if err == nil || !strings.Contains(err.Error(), `unknown field "date"`) {
		t.Errorf("show-only field on a hand-written child: err = %v", err)
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
