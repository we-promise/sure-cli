package output

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// Sure's error message is user-influenced text: the copy on stderr must not
// carry terminal escape sequences, while the JSON envelope keeps it as sent.
func TestFail_StderrCarriesNoControlCharacters(t *testing.T) {
	const message = "\x1b]52;c;ZWNobyBwd24=\x07\x1b[2Jgone\u009b"
	if os.Getenv("SURE_CLI_TEST_FAIL") != "" {
		Fail("not_found", message, nil)
	}
	child := exec.Command(os.Args[0], "-test.run=^TestFail_StderrCarriesNoControlCharacters$")
	child.Env = append(os.Environ(), "SURE_CLI_TEST_FAIL=1")
	var stdout, stderr bytes.Buffer
	child.Stdout, child.Stderr = &stdout, &stderr
	if err := child.Run(); err == nil {
		t.Fatal("Fail exited 0")
	}
	if got := stderr.String(); got != "]52;c;ZWNobyBwd24=[2Jgone\n" {
		t.Fatalf("stderr = %q, want the message without control characters", got)
	}
	if !strings.Contains(stdout.String(), `\u001b]52`) {
		t.Fatalf("stdout = %s, want the JSON envelope to keep the message as sent", stdout.String())
	}
}
