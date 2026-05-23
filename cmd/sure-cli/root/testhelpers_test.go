package root

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/spf13/cobra"
)

// findSub looks up cmd's subcommand by name and fails the test if not found.
// Lives here (not next to any single subcommand's tests) because it is used
// across the entire root package's test suite.
func findSub(t *testing.T, cmd *cobra.Command, name string) *cobra.Command {
	t.Helper()
	sub, _, err := cmd.Find([]string{name})
	if err != nil {
		t.Fatalf("find %q: %v", name, err)
	}
	return sub
}

// captureStdout swaps os.Stdout for a pipe, runs fn, and returns whatever was
// written. The writer goroutine signals via channel for race-free happens-before
// (verified clean under `go test -race`).
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w

	done := make(chan struct{})
	var buf bytes.Buffer
	go func() {
		_, _ = io.Copy(&buf, r)
		close(done)
	}()

	fn()

	_ = w.Close()
	<-done
	os.Stdout = orig
	return buf.String()
}
