package schema

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// ValidateFile validates a JSON file against a JSON schema file.
func ValidateFile(schemaPath, jsonPath string) error {
	c := jsonschema.NewCompiler()
	// Use filesystem loader so we can compile local schema paths.
	c.UseLoader(&jsonschema.FileLoader{})

	absSchema, _ := filepath.Abs(schemaPath)
	s, err := c.Compile("file://" + absSchema)
	if err != nil {
		return fmt.Errorf("compile schema: %w", err)
	}

	b, err := os.ReadFile(jsonPath)
	if err != nil {
		return err
	}
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	if err := s.Validate(v); err != nil {
		return fmt.Errorf("validate: %w", err)
	}
	return nil
}
