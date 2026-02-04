package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dgilperez/sure-cli/internal/schema"
)

func main() {
	schemaPath := flag.String("schema", "", "schema path")
	jsonPath := flag.String("json", "", "json file path")
	flag.Parse()
	if *schemaPath == "" || *jsonPath == "" {
		fmt.Fprintln(os.Stderr, "--schema and --json are required")
		os.Exit(2)
	}
	if err := schema.ValidateFile(*schemaPath, *jsonPath); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	// If this is an envelope and meta.schema is present, also validate `data`.
	b, err := os.ReadFile(*jsonPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	var env struct {
		Data any `json:"data"`
		Meta struct {
			Schema string `json:"schema"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(b, &env); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if env.Meta.Schema != "" && env.Data != nil {
		// Resolve schema path relative to repo root (current working directory).
		p := env.Meta.Schema
		if !filepath.IsAbs(p) {
			p = filepath.Clean(p)
		}
		if err := schema.ValidateValue(p, env.Data); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
}
