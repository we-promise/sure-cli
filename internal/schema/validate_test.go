package schema

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateExamples(t *testing.T) {
	if err := ValidateExamples("../.."); err != nil {
		t.Fatal(err)
	}
}

func TestValidateExamples_RejectsDataThatBreaksItsSchema(t *testing.T) {
	root := t.TempDir()
	write := func(rel, content string) {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	envelope, err := os.ReadFile("../../" + SchemaDir + "/envelope.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	write(SchemaDir+"/envelope.schema.json", string(envelope))
	write(SchemaDir+"/thing.schema.json", `{"type":"object","required":["id"]}`)
	write(SchemaDir+"/other.schema.json", `{"type":"object"}`)
	write(ExampleDir+"/thing.json", `{"data":{"name":"no id"}}`)
	write(ExampleDir+"/orphan.json", `{"data":{}}`)

	err = ValidateExamples(root)
	if err == nil {
		t.Fatal("ValidateExamples accepted data that breaks its schema")
	}
	for _, want := range []string{"thing.json", "other.json", "orphan.json"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error does not mention %s: %v", want, err)
		}
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
