package root

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestImportsList_Flags(t *testing.T) {
	cmd := newImportsCmd()

	list, _, err := cmd.Find([]string{"list"})
	if err != nil {
		t.Fatalf("find list subcommand: %v", err)
	}

	// Verify expected flags exist
	expectedFlags := []string{"status", "type", "page", "per-page"}
	for _, name := range expectedFlags {
		if list.Flags().Lookup(name) == nil {
			t.Fatalf("expected flag %q to exist", name)
		}
	}

	// Verify usage contains expected flags
	s := list.Flags().FlagUsages()
	if !strings.Contains(s, "status") {
		t.Fatalf("expected status in usage")
	}
}

func TestImportsShow_Args(t *testing.T) {
	cmd := newImportsCmd()

	show, _, err := cmd.Find([]string{"show"})
	if err != nil {
		t.Fatalf("find show subcommand: %v", err)
	}

	// Verify it requires exactly 1 argument
	if show.Args == nil {
		t.Fatal("expected Args validator to be set")
	}
}

func TestImportsCreate_Flags(t *testing.T) {
	cmd := newImportsCmd()

	create, _, err := cmd.Find([]string{"create"})
	if err != nil {
		t.Fatalf("find create subcommand: %v", err)
	}

	// Verify expected flags exist
	expectedFlags := []string{"file", "raw-file-content", "type", "account-id", "publish", "date-col-label", "amount-col-label", "rows-to-skip", "apply"}
	for _, name := range expectedFlags {
		if create.Flags().Lookup(name) == nil {
			t.Fatalf("expected flag %q to exist", name)
		}
	}
	// Sure's imports controller neither permits nor reads format/source.
	for _, name := range []string{"file-format", "source"} {
		if create.Flags().Lookup(name) != nil {
			t.Fatalf("flag %q must not exist: Sure ignores it", name)
		}
	}

	s := create.Flags().FlagUsages()
	if !strings.Contains(s, "file") {
		t.Fatalf("expected file in usage")
	}
}

func TestImportsCreate_NoFormatCollision(t *testing.T) {
	cmd := newImportsCmd()

	create, _, err := cmd.Find([]string{"create"})
	if err != nil {
		t.Fatalf("find create subcommand: %v", err)
	}

	// A local --format would shadow the global output flag.
	if create.Flags().Lookup("format") != nil {
		t.Fatal("should not have --format flag (collides with the global output flag)")
	}
}

func TestImportsRows_Flags(t *testing.T) {
	cmd := newImportsCmd()

	rows, _, err := cmd.Find([]string{"rows"})
	if err != nil {
		t.Fatalf("find rows subcommand: %v", err)
	}

	for _, name := range []string{"page", "per-page"} {
		if rows.Flags().Lookup(name) == nil {
			t.Fatalf("expected flag %q to exist", name)
		}
	}

	// Verify it requires exactly 1 argument
	if rows.Args == nil {
		t.Fatal("expected Args validator to be set")
	}
}

func TestBuildImportCreatePayload_ValidFile(t *testing.T) {
	// Create temp file
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.csv")
	if err := os.WriteFile(tmpFile, []byte("test,data"), 0644); err != nil {
		t.Fatalf("create temp file: %v", err)
	}

	opts := importCreateOpts{
		File:         tmpFile,
		importConfig: importConfig{AccountID: "acc-123"},
	}

	payload, err := buildImportCreatePayload(opts)
	if err != nil {
		t.Fatalf("buildImportCreatePayload: %v", err)
	}

	if payload.FilePath != tmpFile {
		t.Errorf("expected FilePath %q, got %q", tmpFile, payload.FilePath)
	}
	for _, key := range []string{"format", "source", "type"} {
		if _, has := payload.Fields[key]; has {
			t.Errorf("field %q must not be sent for a CSV file: %#v", key, payload.Fields)
		}
	}
	if payload.Fields["account_id"] != "acc-123" {
		t.Errorf("expected account_id 'acc-123', got %q", payload.Fields["account_id"])
	}
}

func TestBuildImportCreatePayload_InfersSureImportFromExtension(t *testing.T) {
	tmpDir := t.TempDir()
	for _, name := range []string{"backup.ndjson", "backup.JSON"} {
		tmpFile := filepath.Join(tmpDir, name)
		if err := os.WriteFile(tmpFile, []byte("{}"), 0644); err != nil {
			t.Fatalf("create temp file: %v", err)
		}
		payload, err := buildImportCreatePayload(importCreateOpts{File: tmpFile})
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if payload.Fields["type"] != "SureImport" {
			t.Errorf("%s: expected inferred type SureImport, got %q", name, payload.Fields["type"])
		}
		if _, has := payload.Fields["format"]; has {
			t.Errorf("%s: format must not be sent", name)
		}
	}

	// An explicit --type wins over the extension.
	tmpFile := filepath.Join(tmpDir, "rows.json")
	if err := os.WriteFile(tmpFile, []byte("{}"), 0644); err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	payload, err := buildImportCreatePayload(importCreateOpts{File: tmpFile, Type: "TransactionImport"})
	if err != nil {
		t.Fatal(err)
	}
	if payload.Fields["type"] != "TransactionImport" {
		t.Errorf("expected explicit type to win, got %q", payload.Fields["type"])
	}
}

func TestBuildImportCreatePayload_RowsToSkip(t *testing.T) {
	payload, err := buildImportCreatePayload(importCreateOpts{RawFileContent: "a,b", importConfig: importConfig{RowsToSkip: "2"}})
	if err != nil {
		t.Fatal(err)
	}
	if payload.Fields["rows_to_skip"] != "2" {
		t.Fatalf("rows_to_skip = %q, want \"2\"", payload.Fields["rows_to_skip"])
	}
	for _, bad := range []string{"-1", "x", "1.5"} {
		if _, err := buildImportCreatePayload(importCreateOpts{RawFileContent: "a,b", importConfig: importConfig{RowsToSkip: bad}}); err == nil {
			t.Fatalf("expected err for rows-to-skip %q", bad)
		}
	}
}

func TestBuildImportCreatePayload_MissingFile(t *testing.T) {
	opts := importCreateOpts{
		File: "",
	}

	_, err := buildImportCreatePayload(opts)
	if err == nil {
		t.Fatal("expected error for missing file or raw content")
	}
}

func TestBuildImportCreatePayload_SureImportRawContent(t *testing.T) {
	opts := importCreateOpts{
		Type:           "SureImport",
		RawFileContent: `{"type":"sure_export"}`,
		Publish:        true,
	}

	payload, err := buildImportCreatePayload(opts)
	if err != nil {
		t.Fatalf("buildImportCreatePayload: %v", err)
	}
	if payload.Raw != `{"type":"sure_export"}` || payload.FilePath != "" {
		t.Fatal("expected raw content to be preserved")
	}
	if payload.Fields["type"] != "SureImport" {
		t.Fatalf("expected type SureImport, got %q", payload.Fields["type"])
	}
	if payload.Fields["publish"] != "true" {
		t.Fatalf("expected publish=true, got %q", payload.Fields["publish"])
	}
}

func TestBuildImportCreatePayload_NonexistentFile(t *testing.T) {
	opts := importCreateOpts{
		File: "/nonexistent/path/to/file.csv",
	}

	_, err := buildImportCreatePayload(opts)
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestBuildImportCreatePayload_Directory(t *testing.T) {
	tmpDir := t.TempDir()

	opts := importCreateOpts{
		File: tmpDir, // directory, not file
	}

	_, err := buildImportCreatePayload(opts)
	if err == nil {
		t.Fatal("expected error for directory")
	}
}

// Sure turns a type it does not know into a TransactionImport, so a typo
// would import the file as transactions. The CLI refuses it instead.
func TestBuildImportCreatePayload_UnknownTypeFails(t *testing.T) {
	_, err := buildImportCreatePayload(importCreateOpts{RawFileContent: "a,b", Type: "TradeImprot"})
	if err == nil || !strings.Contains(err.Error(), "--type") || !strings.Contains(err.Error(), "TradeImport") {
		t.Fatalf("error = %v, want one naming --type and the valid types", err)
	}
	for _, typ := range importTypes {
		if _, err := buildImportCreatePayload(importCreateOpts{RawFileContent: "a,b", Type: typ}); err != nil {
			t.Errorf("type %s: %v", typ, err)
		}
	}
}

func TestImportsList_UnknownTypeFailsValidation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	t.Cleanup(srv.Close)
	cfg := apiKeyConfig(t, t.TempDir(), srv.URL)
	if ce := cliError(t, "--config", cfg, "imports", "list", "--type", "Transaction"); ce == nil || ce.Code != "validation_failed" || !strings.Contains(ce.Message, "--type") {
		t.Fatalf("error = %v, want validation_failed naming --type", ce)
	}
	if ce := cliError(t, "--config", cfg, "imports", "list", "--type", "TradeImport"); ce != nil {
		t.Fatalf("TradeImport: %v", ce)
	}
}
