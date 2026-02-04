package schema

import (
	"fmt"
	"path/filepath"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// ValidateValue validates an in-memory value against a JSON schema file.
func ValidateValue(schemaPath string, v any) error {
	c := jsonschema.NewCompiler()
	c.UseLoader(&jsonschema.FileLoader{})

	absSchema, _ := filepath.Abs(schemaPath)
	s, err := c.Compile("file://" + absSchema)
	if err != nil {
		return fmt.Errorf("compile schema: %w", err)
	}
	if err := s.Validate(v); err != nil {
		return fmt.Errorf("validate: %w", err)
	}
	return nil
}
