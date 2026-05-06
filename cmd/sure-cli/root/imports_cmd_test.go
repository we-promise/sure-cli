package root

import (
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
	expectedFlags := []string{"file", "raw-file-content", "type", "file-format", "source", "account-id", "publish", "date-col-label", "amount-col-label", "apply"}
	for _, name := range expectedFlags {
		if create.Flags().Lookup(name) == nil {
			t.Fatalf("expected flag %q to exist", name)
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

	// Verify we use --file-format, not --format (to avoid collision with global output flag)
	if create.Flags().Lookup("format") != nil {
		t.Fatal("should not have --format flag (use --file-format to avoid collision with global output flag)")
	}
	if create.Flags().Lookup("file-format") == nil {
		t.Fatal("expected --file-format flag to exist")
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
		File:      tmpFile,
		Source:    "test-source",
		AccountID: "acc-123",
	}

	payload, err := buildImportCreatePayload(opts)
	if err != nil {
		t.Fatalf("buildImportCreatePayload: %v", err)
	}

	if payload.FilePath != tmpFile {
		t.Errorf("expected FilePath %q, got %q", tmpFile, payload.FilePath)
	}
	if payload.FileField != "file" {
		t.Errorf("expected FileField 'file', got %q", payload.FileField)
	}
	if payload.Fields["format"] != "csv" {
		t.Errorf("expected auto-detected format 'csv', got %q", payload.Fields["format"])
	}
	if payload.Fields["source"] != "test-source" {
		t.Errorf("expected source 'test-source', got %q", payload.Fields["source"])
	}
	if payload.Fields["account_id"] != "acc-123" {
		t.Errorf("expected account_id 'acc-123', got %q", payload.Fields["account_id"])
	}
}

func TestBuildImportCreatePayload_ExplicitFormat(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "data.txt")
	if err := os.WriteFile(tmpFile, []byte("test"), 0644); err != nil {
		t.Fatalf("create temp file: %v", err)
	}

	opts := importCreateOpts{
		File:       tmpFile,
		FileFormat: "ofx",
	}

	payload, err := buildImportCreatePayload(opts)
	if err != nil {
		t.Fatalf("buildImportCreatePayload: %v", err)
	}

	// Explicit format should override extension
	if payload.Fields["format"] != "ofx" {
		t.Errorf("expected explicit format 'ofx', got %q", payload.Fields["format"])
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
	if payload.RawFileContent == "" {
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
