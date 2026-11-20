package root

import (
	"encoding/json"
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// TestOrphanCommandsRegistered checks the registration of commands no other
// test reaches: deleting their AddCommand line in root.go would still compile.
func TestOrphanCommandsRegistered(t *testing.T) {
	root := New()
	cases := []struct {
		path []string
		want string
	}{
		{[]string{"whoami"}, "whoami"},
		{[]string{"sync"}, "sync"},
		{[]string{"refresh"}, "refresh"},
		{[]string{"login"}, "login"},
		{[]string{"status"}, "status"},
		{[]string{"export"}, "export"},
		{[]string{"export", "transactions"}, "transactions"},
		{[]string{"transactions", "delete"}, "delete"},
		{[]string{"insights"}, "insights"},
		{[]string{"insights", "list"}, "list"},
		{[]string{"insights", "ack"}, "ack"},
		{[]string{"analyze"}, "analyze"},
		{[]string{"analyze", "leaks"}, "leaks"},
		{[]string{"analyze", "runway"}, "runway"},
		{[]string{"propose"}, "propose"},
		{[]string{"propose", "rules"}, "rules"},
	}
	for _, c := range cases {
		got, _, err := root.Find(c.path)
		if err != nil {
			t.Fatalf("path %v not registered: %v", c.path, err)
		}
		if got.Name() != c.want {
			t.Fatalf("path %v resolved to %q, want %q", c.path, got.Name(), c.want)
		}
	}
}

// TestMain lets a test re-run this binary as the CLI, to observe exit codes.
// runCLI runs the CLI in a child process and returns stdout and exit code.
func runCLI(t *testing.T, args string) (string, int) {
	t.Helper()
	child, stdout := startCLI(t, append([]string{"--config", filepath.Join(t.TempDir(), "none.yaml")}, strings.Fields(args)...)...)
	err := child.Wait()
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return stdout.String(), exit.ExitCode()
	}
	if err != nil {
		t.Fatalf("run %q: %v", args, err)
	}
	return stdout.String(), 0
}

func TestUnknownSubcommandsFailWithEnvelope(t *testing.T) {
	cases := []struct {
		args, command, hint string
	}{
		{"insights subscriptions", "insights subscriptions", "data.monthly.subscriptions"},
		{"insights fees", "insights fees", "insights list"},
		// Flags the removed command had fail to parse on its group first.
		{"insights leaks --months 3", "insights leaks", "analyze leaks"},
		{"plan runway --account-id x", "plan runway", "analyze runway"},
		{"plan budget", "plan budget", "budgets list"},
		{"plan forecast", "plan forecast", "insights list"},
		{"plan", "plan", "analyze runway"},
		{"config heuristics", "config heuristics", "analyze leaks"},
		{"config fee-keywords", "config fee-keywords", "insights list"},
		{"analyze nope", "analyze nope", ""},
		{"insigts list", "insigts", ""},
	}
	for _, c := range cases {
		t.Run(c.args, func(t *testing.T) {
			out, code := runCLI(t, c.args)
			if code != 1 {
				t.Fatalf("exit code = %d, want 1\nout=%s", code, out)
			}
			var env struct {
				Error *struct {
					Code    string         `json:"code"`
					Message string         `json:"message"`
					Details map[string]any `json:"details"`
				} `json:"error"`
			}
			if err := json.Unmarshal([]byte(out), &env); err != nil || env.Error == nil {
				t.Fatalf("want an error envelope, got %q (%v)", out, err)
			}
			e := env.Error
			if e.Code != "unknown_command" || e.Details["command"] != c.command {
				t.Fatalf("error = %+v", e)
			}
			hint, _ := e.Details["hint"].(string)
			if c.hint == "" && hint != "" || !strings.Contains(hint, c.hint) || !strings.Contains(e.Message, c.hint) {
				t.Fatalf("hint = %q message = %q, want them to mention %q", hint, e.Message, c.hint)
			}
		})
	}
}

func TestUnknownFlagOnACommandFailsValidation(t *testing.T) {
	// The flag error of a real command is about the flag, not an unknown
	// command named after its positional argument.
	for _, c := range []struct{ args, flag string }{
		{"transactions update abc --currency EUR", "--currency"},
		{"transactions show abc --bogus", "--bogus"},
		{"insights ack 8c1d0b2e-1f1a-4c55-9a51-3d2f5e7b9a10 --bogus", "--bogus"},
		{"accounts list --bogus", "--bogus"},
		// Flags and argument counts cobra rejects before any command runs.
		{"--bogus accounts list", "--bogus"},
		{"accounts --bogus list", "--bogus"},
		{"transactions show", "accepts 1 arg"},
	} {
		t.Run(c.args, func(t *testing.T) {
			out, code := runCLI(t, c.args)
			var env struct {
				Error *struct {
					Code    string `json:"code"`
					Message string `json:"message"`
				} `json:"error"`
			}
			if err := json.Unmarshal([]byte(out), &env); err != nil || env.Error == nil {
				t.Fatalf("want an error envelope, got %q (%v)", out, err)
			}
			if code != 1 || env.Error.Code != "validation_failed" || !strings.Contains(env.Error.Message, c.flag) {
				t.Fatalf("exit %d, error %+v; want validation_failed naming %s", code, env.Error, c.flag)
			}
		})
	}
}

func TestUnknownCommandSuggestsNearNames(t *testing.T) {
	ce := unknownCommandError(New(), []string{"insigts"})
	if s, _ := ce.Details["suggestions"].([]string); len(s) != 1 || s[0] != "insights" {
		t.Fatalf("details = %#v", ce.Details)
	}
}

func TestBareGroupPrintsHelp(t *testing.T) {
	out, code := runCLI(t, "analyze")
	if code != 0 || !strings.Contains(out, "Available Commands") || !strings.Contains(out, "leaks") {
		t.Fatalf("exit %d, out=%q", code, out)
	}
}

func TestEveryCommandGroupIsGuarded(t *testing.T) {
	var walk func(*cobra.Command)
	walk = func(c *cobra.Command) {
		if c.HasSubCommands() && c.RunE == nil && c.Run == nil {
			t.Errorf("%q has subcommands but would print help and exit 0 on an unknown one", c.CommandPath())
		}
		for _, sub := range c.Commands() {
			walk(sub)
		}
	}
	walk(New())
}
