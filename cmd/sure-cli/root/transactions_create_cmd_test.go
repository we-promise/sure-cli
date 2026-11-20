package root

import (
	"reflect"
	"testing"
)

func validTxCreateOpts() txCreateOpts {
	return txCreateOpts{
		AccountID: "acc",
		Date:      "2026-02-04",
		Amount:    "1.23",
		Nature:    "expense",
		Name:      "coffee",
	}
}

func TestBuildTxCreatePayload_Valid(t *testing.T) {
	p, err := buildTxCreatePayload(validTxCreateOpts())
	if err != nil {
		t.Fatalf("expected nil err, got %v", err)
	}
	tx := p["transaction"].(map[string]any)
	want := map[string]any{
		"account_id": "acc",
		"date":       "2026-02-04",
		"amount":     "1.23",
		"nature":     "expense",
		"name":       "coffee",
	}
	if !reflect.DeepEqual(tx, want) {
		t.Fatalf("payload = %#v, want %#v", tx, want)
	}
}

// Sure stores entries as decimal(19,4): the amount the user typed is sent
// verbatim, never rounded through a float.
func TestBuildTxCreatePayload_SendsAmountTextVerbatim(t *testing.T) {
	for _, amount := range []string{"0.1234", "1234567890.5", "12", "0.10"} {
		o := validTxCreateOpts()
		o.Amount = amount
		p, err := buildTxCreatePayload(o)
		if err != nil {
			t.Fatalf("%q: unexpected err: %v", amount, err)
		}
		if got := p["transaction"].(map[string]any)["amount"]; got != amount {
			t.Fatalf("amount = %v, want %q", got, amount)
		}
	}
}

func TestBuildTxCreatePayload_RejectsNonDecimalAmount(t *testing.T) {
	for _, amount := range []string{"", "0", "0.00", "-12.34", "+1", "1e3", "1,50", "abc", ".5", "NaN"} {
		o := validTxCreateOpts()
		o.Amount = amount
		if _, err := buildTxCreatePayload(o); err == nil {
			t.Fatalf("expected err for amount %q", amount)
		}
	}
}

func TestBuildTxCreatePayload_OptionalFields(t *testing.T) {
	o := validTxCreateOpts()
	o.Notes = "beans"
	o.Currency = "USD"
	o.CategoryID = "cat"
	o.MerchantID = "mer"
	o.TagIDs = []string{"t1", "t2"}
	o.ExternalID = "bank-42"
	o.Source = "my-importer"
	o.UserModified = true
	p, err := buildTxCreatePayload(o)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	tx := p["transaction"].(map[string]any)
	want := map[string]any{
		"account_id":    "acc",
		"date":          "2026-02-04",
		"amount":        "1.23",
		"nature":        "expense",
		"name":          "coffee",
		"notes":         "beans",
		"currency":      "USD",
		"category_id":   "cat",
		"merchant_id":   "mer",
		"tag_ids":       []string{"t1", "t2"},
		"external_id":   "bank-42",
		"source":        "my-importer",
		"user_modified": true,
	}
	if !reflect.DeepEqual(tx, want) {
		t.Fatalf("payload = %#v, want %#v", tx, want)
	}
}

// Sure answers 422 "Source requires external_id"; fail before the request.
func TestBuildTxCreatePayload_SourceRequiresExternalID(t *testing.T) {
	o := validTxCreateOpts()
	o.Source = "my-importer"
	if _, err := buildTxCreatePayload(o); err == nil {
		t.Fatal("expected err for --source without --external-id")
	}
}

func TestBuildTxCreatePayload_RequiresAccount(t *testing.T) {
	o := validTxCreateOpts()
	o.AccountID = ""
	if _, err := buildTxCreatePayload(o); err == nil {
		t.Fatalf("expected err")
	}
}

func TestBuildTxCreatePayload_RequiresName(t *testing.T) {
	o := validTxCreateOpts()
	o.Name = ""
	if _, err := buildTxCreatePayload(o); err == nil {
		t.Fatalf("expected err")
	}
}

func TestBuildTxCreatePayload_ValidatesDate(t *testing.T) {
	o := validTxCreateOpts()
	o.Date = "nope"
	if _, err := buildTxCreatePayload(o); err == nil {
		t.Fatalf("expected err")
	}
}

func TestBuildTxCreatePayload_ValidatesNature(t *testing.T) {
	o := validTxCreateOpts()
	o.Nature = "weird"
	if _, err := buildTxCreatePayload(o); err == nil {
		t.Fatalf("expected err")
	}
}

func TestTransactionsCreate_Flags(t *testing.T) {
	create := findSub(t, findSub(t, New(), "transactions"), "create")
	for _, name := range []string{"account-id", "date", "amount", "nature", "name", "notes", "currency", "category-id", "merchant-id", "tag-id", "external-id", "source", "user-modified", "apply"} {
		if create.Flags().Lookup(name) == nil {
			t.Fatalf("expected --%s flag", name)
		}
	}
}
