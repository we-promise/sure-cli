package root

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

// Every error.code the CLI emits comes from the catalogue in
// internal/errors, so no call site may pass a string literal as the code.
func TestErrorCodesComeFromTheCatalogue(t *testing.T) {
	codeTakers := map[string]bool{"output.Fail": true, "errs.New": true, "errs.Wrap": true}
	fset := token.NewFileSet()
	err := filepath.WalkDir("../../..", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || len(call.Args) == 0 {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			pkg, ok := sel.X.(*ast.Ident)
			if !ok || !codeTakers[pkg.Name+"."+sel.Sel.Name] {
				return true
			}
			if lit, ok := call.Args[0].(*ast.BasicLit); ok && lit.Kind == token.STRING {
				t.Errorf("%s: %s.%s(%s, ...) uses a literal code; add it to internal/errors", fset.Position(call.Pos()), pkg.Name, sel.Sel.Name, lit.Value)
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestInputErrorsAreValidationFailed(t *testing.T) {
	for _, args := range []string{
		"export transactions --out-format xml", // rejected before any request
		"analyze runway",
		"analyze leaks --months 0",
	} {
		out, code := runCLI(t, args)
		var env struct {
			Error struct{ Code string } `json:"error"`
		}
		if err := json.Unmarshal([]byte(out), &env); err != nil || code != 1 || env.Error.Code != "validation_failed" {
			t.Errorf("%s: exit %d, out %s; want validation_failed", args, code, out)
		}
	}
}
