package root

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/we-promise/sure-cli/internal/api"
	"github.com/we-promise/sure-cli/internal/config"
)

// apiClientFor returns a fresh api.Client targeting the given test server URL
// by wiring viper directly (mirrors internal/api's test client).
func apiClientFor(t *testing.T, url string) *api.Client {
	t.Helper()
	viper.Reset()
	viper.Set("api_url", url)
	viper.Set("auth.mode", "api_key")
	viper.Set("auth.api_key", "test-key")
	_ = config.Init("/tmp/sure-cli-test-does-not-exist.yaml")
	return api.New()
}

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
