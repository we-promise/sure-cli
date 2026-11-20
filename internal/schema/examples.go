package schema

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Directories, relative to the repository root, of the command schemas and
// of one example envelope per schema.
const (
	SchemaDir  = "docs/schemas/v1"
	ExampleDir = "docs/examples"
)

// ValidateExamples checks that every command schema under root has an
// example docs/examples/<name>.json, that each example is a valid envelope,
// that its data matches <name>.schema.json, and that a meta.schema it
// declares names that schema. Envelope schema aside, an example without a
// schema is an error too.
func ValidateExamples(root string) error {
	schemas, err := filepath.Glob(filepath.Join(root, SchemaDir, "*.schema.json"))
	if err != nil {
		return err
	}
	examples, err := filepath.Glob(filepath.Join(root, ExampleDir, "*.json"))
	if err != nil {
		return err
	}
	envelope := filepath.Join(root, SchemaDir, "envelope.schema.json")
	covered := map[string]bool{}
	var errs []error
	for _, s := range schemas {
		if s == envelope {
			continue
		}
		name := strings.TrimSuffix(filepath.Base(s), ".schema.json")
		example := filepath.Join(root, ExampleDir, name+".json")
		covered[example] = true
		if err := validateExample(envelope, s, filepath.ToSlash(filepath.Join(SchemaDir, filepath.Base(s))), example); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", filepath.Join(ExampleDir, name+".json"), err))
		}
	}
	for _, e := range examples {
		if !covered[e] {
			errs = append(errs, fmt.Errorf("%s: no schema %s/%s", filepath.Join(ExampleDir, filepath.Base(e)), SchemaDir, strings.TrimSuffix(filepath.Base(e), ".json")+".schema.json"))
		}
	}
	return errors.Join(errs...)
}

func validateExample(envelope, dataSchema, dataSchemaRef, example string) error {
	if err := ValidateFile(envelope, example); err != nil {
		return err
	}
	b, err := os.ReadFile(example)
	if err != nil {
		return err
	}
	var env struct {
		Data any `json:"data"`
		Meta struct {
			Schema string `json:"schema"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(b, &env); err != nil {
		return err
	}
	if env.Meta.Schema != "" && env.Meta.Schema != dataSchemaRef {
		return fmt.Errorf("meta.schema is %q, want %q", env.Meta.Schema, dataSchemaRef)
	}
	return ValidateValue(dataSchema, env.Data)
}
