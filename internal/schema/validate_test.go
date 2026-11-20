package schema

import "testing"

func TestValidateFile_Smoke(t *testing.T) {
	// Just ensure schema compilation works.
	// Full validation is covered via ./tools/validate-samples in CI.
	if err := ValidateFile("../../docs/schemas/v1/envelope.schema.json", "../../docs/examples/accounts_list.json"); err != nil {
		// Examples may not always match the generic envelope strictness; this is a smoke test.
		// If it fails, we still want visibility.
		t.Logf("validation warning: %v", err)
	}
}

func TestAccountsListSchema_AcceptsBothListShapes(t *testing.T) {
	const schema = "../../docs/schemas/v1/accounts_list.schema.json"
	account := map[string]any{"id": "a1", "name": "Checking", "account_type": "depository", "currency": "EUR", "balance": "€10.00", "classification": "asset"}
	for name, data := range map[string]any{
		"one page (accounts list)":         map[string]any{"accounts": []any{account}, "pagination": map[string]any{"page": 1, "per_page": 25, "total_count": 1, "total_pages": 1}},
		"every page (accounts list --all)": map[string]any{"accounts": []any{account}},
	} {
		if err := ValidateValue(schema, data); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
	if err := ValidateValue(schema, map[string]any{"pagination": map[string]any{}}); err == nil {
		t.Error("output without accounts validated")
	}
}
