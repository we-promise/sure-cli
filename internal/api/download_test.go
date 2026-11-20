package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/spf13/viper"
	errs "github.com/we-promise/sure-cli/internal/errors"
)

func TestGetToFile_WritesBodyOnSuccess(t *testing.T) {
	srv, _ := countingServer(t, http.StatusOK, nil, "PK-export-bytes")
	c := testClient(t, srv.URL)
	dir := t.TempDir()
	out := filepath.Join(dir, "export.zip")

	if _, err := c.GetToFile(context.Background(), "/dl", out, 0); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(out)
	if err != nil || string(got) != "PK-export-bytes" {
		t.Fatalf("file = %q, %v", got, err)
	}
	assertOnlyFiles(t, dir, "export.zip")
}

func TestGetToFile_ErrorBodyIsNotWrittenToOutput(t *testing.T) {
	// Sure answers 409 export_not_ready with a JSON body; it must become an
	// error envelope, never a file masquerading as the export.
	srv, _ := countingServer(t, http.StatusConflict, nil,
		`{"error":"export_not_ready","message":"Export is not ready for download"}`)
	c := testClient(t, srv.URL)
	dir := t.TempDir()
	out := filepath.Join(dir, "export.zip")
	if err := os.WriteFile(out, []byte("previous export"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := c.GetToFile(context.Background(), "/dl", out, 0)
	var ce *errs.CLIError
	if !errors.As(err, &ce) {
		t.Fatalf("want *CLIError, got %T %v", err, err)
	}
	if ce.Code != errs.CodeConflict || ce.Details["api_error"] != "export_not_ready" || ce.Message != "Export is not ready for download" {
		t.Fatalf("got %+v", ce)
	}
	if got, _ := os.ReadFile(out); string(got) != "previous export" {
		t.Fatalf("existing --out file clobbered: %q", got)
	}
	assertOnlyFiles(t, dir, "export.zip")

	// And with no pre-existing file, nothing is created at all.
	fresh := filepath.Join(dir, "new.zip")
	if _, err := c.GetToFile(context.Background(), "/dl", fresh, 0); err == nil {
		t.Fatal("expected error")
	}
	if _, err := os.Stat(fresh); !os.IsNotExist(err) {
		t.Fatalf("error body written to %s (stat err %v)", fresh, err)
	}
	assertOnlyFiles(t, dir, "export.zip")
}

func TestGetToFile_LocalWriteFailureIsExportFailed(t *testing.T) {
	srv, hits := countingServer(t, http.StatusOK, nil, "bytes")
	c := testClient(t, srv.URL)

	_, err := c.GetToFile(context.Background(), "/dl", filepath.Join(t.TempDir(), "missing-dir", "export.zip"), 0)
	var ce *errs.CLIError
	if !errors.As(err, &ce) || ce.Code != errs.CodeExportFailed {
		t.Fatalf("got %v, want export_failed (a local file problem, not a network error)", err)
	}
	if hits[http.MethodGet] != 0 {
		t.Fatal("request sent although the output could not be created")
	}

	dir := t.TempDir() // --out naming an existing directory fails at rename
	_, err = c.GetToFile(context.Background(), "/dl", dir, 0)
	if !errors.As(err, &ce) || ce.Code != errs.CodeExportFailed {
		t.Fatalf("got %v, want export_failed", err)
	}
}

func TestGetToFile_HTMLIsNotSavedAsTheExport(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte("<html>Please log in to the proxy</html>"))
	}))
	t.Cleanup(srv.Close)
	c := testClient(t, srv.URL)
	dir := t.TempDir()

	_, err := c.GetToFile(context.Background(), "/dl", filepath.Join(dir, "export.zip"), 0)
	assertCode(t, err, errs.CodeInvalidResponse)
	assertOnlyFiles(t, dir)
}

func TestGetToFile_ShortBodyIsNotInstalled(t *testing.T) {
	// Sure streams the blob chunked (send_blob_stream), so a storage failure
	// mid-stream ends a well-formed response early. byte_size from
	// GET /family_exports/:id is the only length to check against.
	chunked := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/zip")
		_, _ = w.Write([]byte("PK\x03\x04partial"))
		w.(http.Flusher).Flush()
	}
	closeDelimited := func(w http.ResponseWriter, r *http.Request) {
		conn, buf, _ := w.(http.Hijacker).Hijack()
		_, _ = buf.WriteString("HTTP/1.1 200 OK\r\nContent-Type: application/zip\r\nConnection: close\r\n\r\nPK\x03\x04cut")
		_ = buf.Flush()
		_ = conn.Close()
	}
	for name, h := range map[string]http.HandlerFunc{"chunked": chunked, "close-delimited": closeDelimited} {
		t.Run(name, func(t *testing.T) {
			srv := httptest.NewServer(h)
			t.Cleanup(srv.Close)
			c := testClient(t, srv.URL)
			dir := t.TempDir()
			out := filepath.Join(dir, "export.zip")
			if err := os.WriteFile(out, []byte("previous export"), 0o600); err != nil {
				t.Fatal(err)
			}

			_, err := c.GetToFile(context.Background(), "/dl", out, 4096)
			ce := assertCode(t, err, errs.CodeInvalidResponse)
			if ce.Details["expected_bytes"] != int64(4096) {
				t.Fatalf("details = %v", ce.Details)
			}
			if got, _ := os.ReadFile(out); string(got) != "previous export" {
				t.Fatalf("truncated download installed: %q", got)
			}
			assertOnlyFiles(t, dir, "export.zip")
		})
	}
}

// trickleServer sends chunks of body every interval, or stops after the
// first chunk and holds the connection when stall is set.
func trickleServer(t *testing.T, chunks int, interval time.Duration, stall bool) (*httptest.Server, *int32) {
	t.Helper()
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.Header().Set("Content-Type", "application/zip")
		for i := 0; i < chunks; i++ {
			_, _ = w.Write([]byte(strings.Repeat("Z", 512)))
			w.(http.Flusher).Flush()
			wait := interval
			if stall {
				wait = time.Minute
			}
			select {
			case <-r.Context().Done():
				return
			case <-time.After(wait):
			}
		}
	}))
	t.Cleanup(srv.Close)
	return srv, &calls
}

func timeoutClient(t *testing.T, srvURL string, timeout time.Duration) *Client {
	t.Helper()
	testClient(t, srvURL)
	viper.Set("timeout", timeout.String())
	return New()
}

func TestGetToFile_SlowDownloadOutlivesTheTimeout(t *testing.T) {
	// The timeout bounds silence on the wire, not the whole transfer: an
	// export that keeps flowing must finish, once, however long it takes.
	srv, calls := trickleServer(t, 8, 50*time.Millisecond, false)
	c := timeoutClient(t, srv.URL, 200*time.Millisecond)
	out := filepath.Join(t.TempDir(), "export.zip")

	if _, err := c.GetToFile(context.Background(), "/dl", out, 8*512); err != nil {
		t.Fatalf("download failed: %v", err)
	}
	if got := atomic.LoadInt32(calls); got != 1 {
		t.Fatalf("requests = %d, want 1", got)
	}
}

func TestGetToFile_StalledDownloadTimesOutWithoutRestarting(t *testing.T) {
	srv, calls := trickleServer(t, 8, 0, true)
	c := timeoutClient(t, srv.URL, 150*time.Millisecond)
	dir := t.TempDir()

	_, err := c.GetToFile(context.Background(), "/dl", filepath.Join(dir, "export.zip"), 0)
	assertCode(t, err, errs.CodeTimeout)
	if got := atomic.LoadInt32(calls); got != 1 {
		t.Fatalf("requests = %d, want 1 (a body already flowing is never restarted)", got)
	}
	assertOnlyFiles(t, dir)
}

func TestGetToFile_CancelRemovesThePartialFile(t *testing.T) {
	srv, _ := trickleServer(t, 8, 0, true)
	c := testClient(t, srv.URL)
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(100*time.Millisecond, cancel)

	_, err := c.GetToFile(ctx, "/dl", filepath.Join(dir, "export.zip"), 0)
	assertCode(t, err, errs.CodeCanceled)
	assertOnlyFiles(t, dir)
}

func assertCode(t *testing.T, err error, code string) *errs.CLIError {
	t.Helper()
	ce := CheckResponse(nil, err)
	if err == nil || ce.Code != code {
		t.Fatalf("err = %v, want %s", err, code)
	}
	return ce
}

// assertOnlyFiles fails if dir holds anything besides names (e.g. leftover
// temp files).
func assertOnlyFiles(t *testing.T, dir string, names ...string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != len(names) {
		var got []string
		for _, e := range entries {
			got = append(got, e.Name())
		}
		t.Fatalf("dir contains %v, want %v", got, names)
	}
	for i, e := range entries {
		if e.Name() != names[i] {
			t.Fatalf("dir contains %q, want %v", e.Name(), names)
		}
	}
}
