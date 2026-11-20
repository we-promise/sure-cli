package root

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestMimeForUploadFile(t *testing.T) {
	cases := []struct {
		path string
		want string
	}{
		// Parameter-free types — upstream `include?` allow-lists are exact-match.
		{"data.csv", "text/csv"},
		{"data.CSV", "text/csv"}, // case-insensitive
		{"backup.ndjson", "application/x-ndjson"},
		{"backup.NDJSON", "application/x-ndjson"},
		{"backup.json", "application/json"},
		{"/abs/path/to/data.csv", "text/csv"},
		{"statement.pdf", "application/pdf"},
		{"statement.PDF", "application/pdf"},
		{"statement.xlsx", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"},
		// Unknown extensions fall back to resty auto-detection.
		{"unknown.xyz", ""},
		{"noext", ""},
		{"", ""},
	}
	for _, c := range cases {
		if got := mimeForUploadFile(c.path); got != c.want {
			t.Fatalf("path %q: got %q, want %q", c.path, got, c.want)
		}
	}
}

func TestUploadDryRunBodyNamesTheFileNotItsContent(t *testing.T) {
	file := upload{Fields: map[string]string{"sequence": "1"}, FilePath: "part.ndjson"}
	if got := file.dryRunBody(); !reflect.DeepEqual(got, map[string]any{"sequence": "1", "file": "part.ndjson"}) {
		t.Errorf("file dry run = %#v", got)
	}
	raw := upload{Fields: map[string]string{"type": "SureImport"}, Raw: "{}"}
	if got := raw.dryRunBody(); !reflect.DeepEqual(got, map[string]any{"type": "SureImport", "raw_file_content": "{}"}) {
		t.Errorf("raw dry run = %#v", got)
	}
}

func TestRegularFileRejectsDevicesAndDirectories(t *testing.T) {
	for _, path := range []string{os.DevNull, t.TempDir(), filepath.Join(t.TempDir(), "missing.csv")} {
		if _, err := regularFile(path); err == nil {
			t.Errorf("regularFile(%q) accepted it", path)
		}
	}
	path := filepath.Join(t.TempDir(), "a.csv")
	if err := os.WriteFile(path, []byte("abc"), 0o600); err != nil {
		t.Fatal(err)
	}
	if info, err := regularFile(path); err != nil || info.Size() != 3 {
		t.Errorf("regularFile(regular) = %v, %v", info, err)
	}
}
