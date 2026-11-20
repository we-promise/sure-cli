package root

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestFamilyExportsCommandShape(t *testing.T) {
	cmd := newFamilyExportsCmd()
	for _, args := range [][]string{
		{"list"},
		{"show"},
		{"create"},
		{"download"},
	} {
		if _, _, err := cmd.Find(args); err != nil {
			t.Fatalf("expected family-exports command %v: %v", args, err)
		}
	}

	create, _, err := cmd.Find([]string{"create"})
	if err != nil {
		t.Fatalf("find create command: %v", err)
	}
	if create.Flags().Lookup("apply") == nil {
		t.Fatal("expected create --apply flag")
	}

	download, _, err := cmd.Find([]string{"download"})
	if err != nil {
		t.Fatalf("find download command: %v", err)
	}
	if download.Flags().Lookup("out") == nil {
		t.Fatal("expected download --out flag")
	}
}

// exportServer serves family export abc: show with byteSize, and download
// via body.
func exportServer(t *testing.T, byteSize int, body http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/family_exports/abc":
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintf(w, `{"data":{"id":"abc","downloadable":true,"file":{"attached":true,"byte_size":%d}}}`, byteSize)
		case "/api/v1/family_exports/abc/download":
			body(w, r)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func apiKeyConfig(t *testing.T, dir, srvURL string) string {
	t.Helper()
	cfg := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(cfg, []byte("api_url: "+srvURL+"\nauth:\n  mode: api_key\n  api_key: k\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return cfg
}

func TestFamilyExportsDownload_ChecksTheExportByteSize(t *testing.T) {
	for name, tc := range map[string]struct {
		byteSize int
		wantCode string
	}{
		"complete":  {byteSize: len("PK-export"), wantCode: ""},
		"truncated": {byteSize: 4096, wantCode: "invalid_response"},
	} {
		t.Run(name, func(t *testing.T) {
			srv := exportServer(t, tc.byteSize, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/zip")
				_, _ = w.Write([]byte("PK-export"))
			})
			dir := t.TempDir()
			out := filepath.Join(dir, "export.zip")
			child, stdout := startCLI(t, "--config", apiKeyConfig(t, dir, srv.URL), "family-exports", "download", "abc", "--out", out)
			_ = child.Wait()

			var env struct {
				Error *struct {
					Code string `json:"code"`
				} `json:"error"`
			}
			if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
				t.Fatalf("stdout %q: %v", stdout, err)
			}
			got := ""
			if env.Error != nil {
				got = env.Error.Code
			}
			if got != tc.wantCode {
				t.Fatalf("error code = %q, want %q (stdout %s)", got, tc.wantCode, stdout)
			}
			_, err := os.Stat(out)
			if installed := err == nil; installed != (tc.wantCode == "") {
				t.Fatalf("export installed = %v", installed)
			}
		})
	}
}
