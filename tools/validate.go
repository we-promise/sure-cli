package main

import (
	"flag"
	"fmt"
	"os"

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
}
