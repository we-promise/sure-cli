//go:build unix

package root

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFamilyExportsDownload_InterruptRemovesThePartialFile(t *testing.T) {
	flowing := make(chan struct{})
	srv := exportServer(t, 1<<20, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/zip")
		_, _ = w.Write([]byte(strings.Repeat("Z", 4096)))
		w.(http.Flusher).Flush()
		close(flowing)
		<-r.Context().Done()
	})
	dir := t.TempDir()
	cfg := apiKeyConfig(t, dir, srv.URL)
	child, stdout := startCLI(t, "--config", cfg, "family-exports", "download", "abc", "--out", filepath.Join(dir, "export.zip"))

	select {
	case <-flowing:
	case <-time.After(10 * time.Second):
		_ = child.Process.Kill()
		t.Fatal("download never started")
	}
	if err := child.Process.Signal(os.Interrupt); err != nil {
		t.Fatal(err)
	}
	_ = child.Wait()

	if !strings.Contains(stdout.String(), `"code": "canceled"`) {
		t.Fatalf("stdout = %s, want a canceled error envelope", stdout)
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if e.Name() != "config.yaml" {
			t.Fatalf("left behind %s", e.Name())
		}
	}
}

func TestFamilyExportsDownload_InterruptDuringTheExportLookup(t *testing.T) {
	asked := make(chan struct{})
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(asked)
		<-release // never answers the metadata GET
	}))
	t.Cleanup(srv.Close)
	t.Cleanup(func() { close(release) })
	dir := t.TempDir()
	child, stdout := startCLI(t, "--config", apiKeyConfig(t, dir, srv.URL), "--timeout", "0", "family-exports", "download", "abc", "--out", filepath.Join(dir, "export.zip"))

	select {
	case <-asked:
	case <-time.After(10 * time.Second):
		_ = child.Process.Kill()
		t.Fatal("lookup never sent")
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
		t.Fatal("Ctrl-C ignored while looking up the export")
	}
	if !strings.Contains(stdout.String(), `"code": "canceled"`) {
		t.Fatalf("stdout = %s, want a canceled error envelope", stdout)
	}
}
