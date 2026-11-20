package root

import (
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/we-promise/sure-cli/internal/config"
	errs "github.com/we-promise/sure-cli/internal/errors"
)

func TestTimeoutFlagSetsTheClientTimeout(t *testing.T) {
	cli := New()
	var got time.Duration
	cli.AddCommand(&cobra.Command{Use: "probe", Run: func(*cobra.Command, []string) { got = config.Timeout() }})
	cli.SetArgs([]string{"--config", filepath.Join(t.TempDir(), "none.yaml"), "--timeout", "2m", "probe"})
	if err := cli.Execute(); err != nil {
		t.Fatal(err)
	}
	if got != 2*time.Minute {
		t.Fatalf("timeout = %v, want 2m", got)
	}
}

// cliError runs sure-cli in a child process and returns the error of its
// envelope, nil when it succeeded.
func cliError(t *testing.T, args ...string) *errs.CLIError {
	t.Helper()
	ce, _ := cliRun(t, args...)
	return ce
}

// cliRun is cliError that also returns the child's stdout.
func cliRun(t *testing.T, args ...string) (*errs.CLIError, string) {
	t.Helper()
	child, stdout := startCLI(t, args...)
	err := child.Wait()
	var env struct {
		Error *errs.CLIError `json:"error"`
	}
	if jerr := json.Unmarshal(stdout.Bytes(), &env); err != nil && (jerr != nil || env.Error == nil) {
		t.Fatalf("%v failed without an error envelope: %v\n%s", args, err, stdout)
	}
	return env.Error, stdout.String()
}
