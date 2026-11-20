//go:build unix

package root

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

func TestExportTransactions_InterruptRemovesThePartialFileAndTheSpool(t *testing.T) {
	asked := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/v1/family_settings":
			fmt.Fprint(w, `{"timezone":"UTC"}`)
		case r.URL.Query().Get("page") == "1":
			fmt.Fprintf(w, `{"transactions":[%s],"pagination":{"total_pages":2}}`, txJSON("t1", "2026-02-01", "a", "standard", "EUR", "1.0", false))
		default:
			close(asked)
			<-r.Context().Done() // page 2 never answers
		}
	}))
	t.Cleanup(srv.Close)
	dir, tmp := t.TempDir(), t.TempDir()
	t.Setenv("TMPDIR", tmp)
	child, stdout := startCLI(t, "--config", apiKeyConfig(t, dir, srv.URL), "--timeout", "0", "export", "transactions", "--out-format", "json", "--out", filepath.Join(dir, "tx.json"))

	select {
	case <-asked:
	case <-time.After(10 * time.Second):
		_ = child.Process.Kill()
		t.Fatal("page 2 never requested")
	}
	if err := child.Process.Signal(os.Interrupt); err != nil {
		t.Fatal(err)
	}
	exited := make(chan struct{})
	go func() { _ = child.Wait(); close(exited) }()
	select {
	case <-exited:
	case <-time.After(5 * time.Second):
		_ = child.Process.Kill()
		t.Fatal("Ctrl-C ignored while reading pages")
	}
	if !strings.Contains(stdout.String(), `"code": "canceled"`) {
		t.Fatalf("stdout = %s, want a canceled error envelope", stdout)
	}
	for _, d := range []string{dir, tmp} {
		entries, _ := os.ReadDir(d)
		for _, e := range entries {
			if e.Name() != "config.yaml" {
				t.Errorf("left behind %s", filepath.Join(d, e.Name()))
			}
		}
	}
}
