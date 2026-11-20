package root

import (
	"reflect"
	"testing"
)

func TestBuildTxUpdatePayload_RequiresAtLeastOneField(t *testing.T) {
	if _, err := buildTxUpdatePayload(txUpdateOpts{}); err == nil {
		t.Fatalf("expected err")
	}
}

func TestBuildTxUpdatePayload_ValidAmountRequiresNature(t *testing.T) {
	if _, err := buildTxUpdatePayload(txUpdateOpts{Amount: "1", Date: "2026-02-04"}); err == nil {
		t.Fatalf("expected err")
	}
}

func TestBuildTxUpdatePayload_Valid(t *testing.T) {
	p, err := buildTxUpdatePayload(txUpdateOpts{Name: "x", Amount: "1.2", Nature: "expense", Date: "2026-02-04"})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	tx := p["transaction"].(map[string]any)
	want := map[string]any{"name": "x", "amount": "1.2", "nature": "expense", "date": "2026-02-04"}
	if !reflect.DeepEqual(tx, want) {
		t.Fatalf("payload = %#v, want %#v", tx, want)
	}
}

func TestBuildTxUpdatePayload_SendsAmountTextVerbatim(t *testing.T) {
	p, err := buildTxUpdatePayload(txUpdateOpts{Amount: "0.1234", Nature: "income"})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got := p["transaction"].(map[string]any)["amount"]; got != "0.1234" {
		t.Fatalf("amount = %v, want \"0.1234\"", got)
	}
}

func TestBuildTxUpdatePayload_RejectsNonDecimalAmount(t *testing.T) {
	for _, amount := range []string{"0", "-1", "1e3", "1,5", "abc"} {
		if _, err := buildTxUpdatePayload(txUpdateOpts{Amount: amount, Nature: "expense"}); err == nil {
			t.Fatalf("expected err for amount %q", amount)
		}
	}
}

func TestBuildTxUpdatePayload_ReferenceFields(t *testing.T) {
	p, err := buildTxUpdatePayload(txUpdateOpts{CategoryID: "cat", MerchantID: "mer", TagIDs: []string{"t1"}, UserModified: true})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	tx := p["transaction"].(map[string]any)
	want := map[string]any{"category_id": "cat", "merchant_id": "mer", "tag_ids": []string{"t1"}, "user_modified": true}
	if !reflect.DeepEqual(tx, want) {
		t.Fatalf("payload = %#v, want %#v", tx, want)
	}
}

// Sure keeps tags when tag_ids is absent and clears them on an explicit
// empty array, so --clear-tags must send [] (not omit the key).
func TestBuildTxUpdatePayload_ClearTags(t *testing.T) {
	p, err := buildTxUpdatePayload(txUpdateOpts{ClearTags: true})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	tags, ok := p["transaction"].(map[string]any)["tag_ids"]
	if !ok {
		t.Fatal("expected tag_ids key")
	}
	if got, ok := tags.([]string); !ok || got == nil || len(got) != 0 {
		t.Fatalf("tag_ids = %#v, want empty non-nil []string", tags)
	}
}

func TestBuildTxUpdatePayload_ClearTagsConflictsWithTagID(t *testing.T) {
	if _, err := buildTxUpdatePayload(txUpdateOpts{ClearTags: true, TagIDs: []string{"t1"}}); err == nil {
		t.Fatal("expected err for --clear-tags with --tag-id")
	}
}

func TestBuildTxUpdatePayload_TagsOmittedWhenNotRequested(t *testing.T) {
	p, err := buildTxUpdatePayload(txUpdateOpts{Name: "x"})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if _, has := p["transaction"].(map[string]any)["tag_ids"]; has {
		t.Fatal("tag_ids must be omitted so Sure keeps the existing tags")
	}
}

// Sure ignores currency on update (entry_params_for_update drops it), so the
// CLI does not offer a flag that would silently do nothing.
func TestTransactionsUpdate_Flags(t *testing.T) {
	update := findSub(t, findSub(t, New(), "transactions"), "update")
	if update.Flags().Lookup("currency") != nil {
		t.Fatal("--currency must not exist on update: Sure ignores it")
	}
	for _, name := range []string{"date", "amount", "nature", "name", "notes", "category-id", "merchant-id", "tag-id", "clear-tags", "user-modified", "apply"} {
		if update.Flags().Lookup(name) == nil {
			t.Fatalf("expected --%s flag", name)
		}
	}
}
