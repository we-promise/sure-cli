//go:build unix

package root

import (
	"strings"
	"testing"
)

func TestExportTransactions_WritesToStdout(t *testing.T) {
	url := exportTxServer(t, txJSON("t1", "2026-02-01", "a", "standard", "EUR", "1.0", false))
	dir := t.TempDir()
	for _, outFormat := range []string{"csv", "json"} {
		child, stdout := startCLI(t, "--config", apiKeyConfig(t, dir, url), "export", "transactions", "--out", "/dev/stdout", "--out-format", outFormat)
		if err := child.Wait(); err != nil {
			t.Fatalf("%s: %v\n%s", outFormat, err, stdout)
		}
		if !strings.Contains(stdout.String(), `"t1"`) && !strings.Contains(stdout.String(), "t1,2026-02-01") {
			t.Fatalf("%s: stdout = %s, want the exported row", outFormat, stdout)
		}
	}
}
