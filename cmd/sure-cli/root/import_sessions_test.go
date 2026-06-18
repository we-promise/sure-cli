package root

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBuildImportSessionCreatePayload_RequiresType(t *testing.T) {
	if _, err := buildImportSessionCreatePayload(importSessionCreateOpts{Type: "   "}); err == nil {
		t.Fatal("expected blank type to be rejected")
	}
}

func TestBuildImportSessionCreatePayload_Minimal(t *testing.T) {
	body, err := buildImportSessionCreatePayload(importSessionCreateOpts{Type: "SureImport"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if body["type"] != "SureImport" {
		t.Fatalf("type = %v", body["type"])
	}
	if _, has := body["client_session_id"]; has {
		t.Fatal("client_session_id should be omitted when empty")
	}
	if _, has := body["expected_chunks"]; has {
		t.Fatal("expected_chunks should be omitted when zero")
	}
}

func TestBuildImportSessionCreatePayload_OptionalFields(t *testing.T) {
	body, err := buildImportSessionCreatePayload(importSessionCreateOpts{
		Type:            "SureImport",
		ClientSessionID: "sess-1",
		ExpectedChunks:  3,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if body["client_session_id"] != "sess-1" {
		t.Fatalf("client_session_id = %v", body["client_session_id"])
	}
	if body["expected_chunks"] != 3 {
		t.Fatalf("expected_chunks = %v", body["expected_chunks"])
	}
}

func TestBuildImportSessionCreatePayload_NegativeExpectedChunks(t *testing.T) {
	if _, err := buildImportSessionCreatePayload(importSessionCreateOpts{Type: "SureImport", ExpectedChunks: -1}); err == nil {
		t.Fatal("expected negative expected-chunks to be rejected")
	}
}

func TestBuildChunkCreate_RequiresSequence(t *testing.T) {
	if _, err := buildChunkCreate(chunkCreateOpts{RawContent: "{}"}); err == nil {
		t.Fatal("expected missing sequence to be rejected")
	}
	if _, err := buildChunkCreate(chunkCreateOpts{Sequence: 0, RawContent: "{}"}); err == nil {
		t.Fatal("expected sequence 0 to be rejected")
	}
}

func TestBuildChunkCreate_RequiresExactlyOneContentSource(t *testing.T) {
	if _, err := buildChunkCreate(chunkCreateOpts{Sequence: 1}); err == nil {
		t.Fatal("expected missing content to be rejected")
	}
	if _, err := buildChunkCreate(chunkCreateOpts{Sequence: 1, File: "a.ndjson", RawContent: "{}"}); err == nil {
		t.Fatal("expected both file and raw-content to be rejected")
	}
}

func TestBuildChunkCreate_RawContentBuildsJSONBody(t *testing.T) {
	plan, err := buildChunkCreate(chunkCreateOpts{Sequence: 2, RawContent: `{"a":1}`, ClientChunkID: "c-2"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plan.Multipart {
		t.Fatal("raw content should not be multipart")
	}
	if plan.JSONBody["sequence"] != 2 {
		t.Fatalf("sequence = %v", plan.JSONBody["sequence"])
	}
	if plan.JSONBody["raw_file_content"] != `{"a":1}` {
		t.Fatalf("raw_file_content = %v", plan.JSONBody["raw_file_content"])
	}
	if plan.JSONBody["client_chunk_id"] != "c-2" {
		t.Fatalf("client_chunk_id = %v", plan.JSONBody["client_chunk_id"])
	}
}

func TestBuildChunkCreate_RejectsNonNDJSONExtension(t *testing.T) {
	if _, err := buildChunkCreate(chunkCreateOpts{Sequence: 1, File: "data.csv"}); err == nil {
		t.Fatal("expected .csv file to be rejected (NDJSON only)")
	}
}

func TestBuildChunkCreate_FileBuildsMultipart(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "chunk.ndjson")
	if err := os.WriteFile(fp, []byte(`{"x":1}`+"\n"), 0o600); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	plan, err := buildChunkCreate(chunkCreateOpts{Sequence: 1, File: fp})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !plan.Multipart {
		t.Fatal("file path should be multipart")
	}
	if plan.FilePath != fp {
		t.Fatalf("FilePath = %q", plan.FilePath)
	}
	if plan.Fields["sequence"] != "1" {
		t.Fatalf("sequence field = %q", plan.Fields["sequence"])
	}
}

func TestBuildChunkCreate_MissingFileIsRejected(t *testing.T) {
	if _, err := buildChunkCreate(chunkCreateOpts{Sequence: 1, File: filepath.Join(t.TempDir(), "nope.ndjson")}); err == nil {
		t.Fatal("expected missing file to be rejected")
	}
}

func TestImportSessionsCommandsRegistered(t *testing.T) {
	root := New()
	cases := []struct {
		path  []string
		flags []string
	}{
		{[]string{"import-sessions", "create"}, []string{"type", "client-session-id", "expected-chunks", "apply"}},
		{[]string{"import-sessions", "show"}, nil},
		{[]string{"import-sessions", "add-chunk"}, []string{"sequence", "client-chunk-id", "file", "raw-content", "apply"}},
		{[]string{"import-sessions", "publish"}, []string{"apply"}},
	}
	for _, c := range cases {
		got, _, err := root.Find(c.path)
		if err != nil {
			t.Fatalf("%v not registered: %v", c.path, err)
		}
		if got.Name() != c.path[len(c.path)-1] {
			t.Fatalf("resolved to %q, want %q", got.Name(), c.path[len(c.path)-1])
		}
		for _, f := range c.flags {
			if got.Flags().Lookup(f) == nil {
				t.Fatalf("%v missing --%s", c.path, f)
			}
		}
	}
}
