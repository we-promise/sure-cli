package api

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	errs "github.com/we-promise/sure-cli/internal/errors"
)

func TestGetToFile_WritesBodyOnSuccess(t *testing.T) {
	srv, _ := countingServer(t, http.StatusOK, nil, "PK-export-bytes")
	c := testClient(t, srv.URL)
	dir := t.TempDir()
	out := filepath.Join(dir, "export.zip")

	if _, err := c.GetToFile("/dl", out); err != nil {
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

	_, err := c.GetToFile("/dl", out)
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
	if _, err := c.GetToFile("/dl", fresh); err == nil {
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

	_, err := c.GetToFile("/dl", filepath.Join(t.TempDir(), "missing-dir", "export.zip"))
	var ce *errs.CLIError
	if !errors.As(err, &ce) || ce.Code != errs.CodeExportFailed {
		t.Fatalf("got %v, want export_failed (a local file problem, not a network error)", err)
	}
	if hits[http.MethodGet] != 0 {
		t.Fatal("request sent although the output could not be created")
	}

	dir := t.TempDir() // --out naming an existing directory fails at rename
	_, err = c.GetToFile("/dl", dir)
	if !errors.As(err, &ce) || ce.Code != errs.CodeExportFailed {
		t.Fatalf("got %v, want export_failed", err)
	}
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
