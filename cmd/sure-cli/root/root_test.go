package root

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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

func TestTimeoutAcceptsBareSeconds(t *testing.T) {
	cli := New()
	var got time.Duration
	cli.AddCommand(&cobra.Command{Use: "probe", Run: func(*cobra.Command, []string) { got = config.Timeout() }})
	cli.SetArgs([]string{"--config", filepath.Join(t.TempDir(), "none.yaml"), "--timeout", "45", "probe"})
	if err := cli.Execute(); err != nil {
		t.Fatal(err)
	}
	if got != 45*time.Second {
		t.Fatalf("timeout = %v, want 45s", got)
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

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	cfg := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(cfg, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return cfg
}

func TestInvalidTimeoutFailsValidation(t *testing.T) {
	for name, args := range map[string][]string{
		"flag unparsable": {"--config", writeConfig(t, ""), "--timeout", "abc", "whoami"},
		"flag negative":   {"--config", writeConfig(t, ""), "--timeout", "-5s", "whoami"},
		"config key":      {"--config", writeConfig(t, "timeout: soon\n"), "whoami"},
		"config set":      {"--config", writeConfig(t, ""), "config", "set", "timeout", "soon"},
	} {
		t.Run(name, func(t *testing.T) {
			if ce := cliError(t, args...); ce == nil || ce.Code != errs.CodeValidation || !strings.Contains(ce.Message, "timeout") {
				t.Fatalf("error = %v, want validation_failed about the timeout", ce)
			}
		})
	}
}

func TestConfigCommandsWorkWithAnInvalidConfig(t *testing.T) {
	cfg := writeConfig(t, "api_url: http://sure.invalid\ntimeout: soon\n")
	for _, args := range [][]string{{"config", "get", "timeout"}, {"config", "set", "timeout", "30"}, {"config", "set", "api_url", "https://sure.invalid"}} {
		if ce := cliError(t, append([]string{"--config", cfg}, args...)...); ce != nil {
			t.Fatalf("%v: %v", args, ce)
		}
	}
	if b, _ := os.ReadFile(cfg); !strings.Contains(string(b), "https://sure.invalid") {
		t.Fatalf("config = %s", b)
	}
}

func TestPlainHTTPToARemoteHostNeedsOptIn(t *testing.T) {
	cfg := writeConfig(t, "api_url: http://sure.invalid\nauth:\n  mode: api_key\n  api_key: k\n")
	if ce := cliError(t, "--config", cfg, "whoami"); ce == nil || ce.Code != errs.CodeValidation || !strings.Contains(ce.Message, "--allow-insecure-http") {
		t.Fatalf("error = %v, want validation_failed naming the opt-in", ce)
	}
	optedIn := writeConfig(t, "api_url: http://sure.invalid\nallow_insecure_http: true\nauth:\n  mode: api_key\n  api_key: k\n")
	for _, args := range [][]string{{"--config", cfg, "--allow-insecure-http", "whoami"}, {"--config", optedIn, "whoami"}} {
		if ce := cliError(t, args...); ce == nil || ce.Code == errs.CodeValidation {
			t.Fatalf("%v: error = %v, want the request to be sent (and fail to connect)", args, ce)
		}
	}
}

func TestCommandsThatSendNothingIgnoreAnInvalidConfig(t *testing.T) {
	cfg := writeConfig(t, "api_url: http://sure.invalid\ntimeout: soon\n")
	for _, args := range [][]string{{"version"}, {"help"}, {"completion", "bash"}, {"accounts"}} {
		if ce := cliError(t, append([]string{"--config", cfg}, args...)...); ce != nil {
			t.Fatalf("%v: %v", args, ce)
		}
	}
	if ce := cliError(t, "--config", cfg, "nope"); ce == nil || ce.Code != errs.CodeUnknownCommand {
		t.Fatalf("unknown command: error = %v, want unknown_command", ce)
	}
}

func TestLoginRejectsAnInvalidConfigBeforePrompting(t *testing.T) {
	cfg := writeConfig(t, "api_url: http://sure.invalid\n")
	// stdin is empty: a prompt would fail with password_read_failed.
	if ce := cliError(t, "--config", cfg, "login", "--email", "a@b.c"); ce == nil || ce.Code != errs.CodeValidation {
		t.Fatalf("error = %v, want validation_failed", ce)
	}
}

// A usage error is one validation_failed envelope on stdout, without cobra's
// usage text on stderr.
func TestUsageErrorsAreOneErrorWithoutUsage(t *testing.T) {
	cfg := writeConfig(t, "api_url: [unclosed\n")
	cases := []struct {
		name, message string
		args          []string
	}{
		{"unreadable config", "read config", []string{"--config", cfg, "accounts", "list"}},
		{"stray argument", "unknown command \"out.csv\"", []string{"export", "transactions", "out.csv"}},
		{"unknown flag", "unknown flag: --nope", []string{"accounts", "list", "--nope"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			child := cliCommand(c.args...)
			var stdout, stderr bytes.Buffer
			child.Stdout, child.Stderr = &stdout, &stderr
			if err := child.Run(); err == nil {
				t.Fatalf("exit 0: %s", stdout.String())
			}
			var env struct {
				Error struct {
					Code    string `json:"code"`
					Message string `json:"message"`
				} `json:"error"`
			}
			if err := json.Unmarshal(stdout.Bytes(), &env); err != nil || env.Error.Code != "validation_failed" || !strings.Contains(env.Error.Message, c.message) {
				t.Fatalf("stdout = %s (%v), want a validation_failed envelope containing %q", stdout.String(), err, c.message)
			}
			if strings.Contains(stderr.String(), "Usage:") {
				t.Fatalf("stderr prints usage:\n%s", stderr.String())
			}
		})
	}
}
