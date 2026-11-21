//go:build unix

package root

import (
	"encoding/json"
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
		// stdout is the file: exactly one document, the envelope goes to stderr.
		switch outFormat {
		case "csv":
			if got := stdout.String(); got != "id,date,name,amount,currency,classification,kind,category,account,merchant,excluded,split_parent\nt1,2026-02-01,a,-1.0,EUR,expense,standard,,Main,,false,false\n" {
				t.Fatalf("csv: stdout = %q, want only the CSV", got)
			}
		case "json":
			var doc struct {
				Count        int              `json:"count"`
				Transactions []map[string]any `json:"transactions"`
			}
			dec := json.NewDecoder(stdout)
			if err := dec.Decode(&doc); err != nil || doc.Count != 1 || dec.More() {
				t.Fatalf("json: stdout is not one export document (err %v, count %d, more %v)", err, doc.Count, dec.More())
			}
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

func TestFamilyExportsDownload_ToStdoutIsOnlyTheArchive(t *testing.T) {
	srv := exportServer(t, len("PK-export"), func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/zip")
		_, _ = w.Write([]byte("PK-export"))
	})
	child, stdout := startCLI(t, "--config", apiKeyConfig(t, t.TempDir(), srv.URL), "family-exports", "download", "abc", "--out", "/dev/stdout")
	if err := child.Wait(); err != nil {
		t.Fatalf("%v\n%s", err, stdout)
	}
	if got := stdout.String(); got != "PK-export" {
		t.Fatalf("stdout = %q, want only the archive bytes", got)
	}
}
