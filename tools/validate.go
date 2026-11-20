// Command validate checks every example envelope under docs/examples
// against the envelope schema and its command's data schema.
package main

import (
	"fmt"
	"os"

	"github.com/we-promise/sure-cli/internal/schema"
)

func main() {
	root := "."
	if len(os.Args) > 1 {
		root = os.Args[1]
	}
	if err := schema.ValidateExamples(root); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("examples valid")
}
