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

func TestEnvelopeSchema_MetaWarnings(t *testing.T) {
	envelope := "../../" + SchemaDir + "/envelope.schema.json"
	warned := map[string]any{"data": map[string]any{}, "meta": map[string]any{"warnings": []any{
		map[string]any{"code": "not_found", "message": "cannot read the family's time zone", "details": map[string]any{"time_zone": "Local"}},
	}}}
	if err := ValidateValue(envelope, warned); err != nil {
		t.Fatal(err)
	}
	broken := map[string]any{"data": map[string]any{}, "meta": map[string]any{"warnings": []any{map[string]any{"message": "no code"}}}}
	if ValidateValue(envelope, broken) == nil {
		t.Fatal("a warning without a code passed")
	}
}
