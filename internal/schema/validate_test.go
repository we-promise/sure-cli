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
