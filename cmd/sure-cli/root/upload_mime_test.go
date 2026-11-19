package root

import "testing"

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
