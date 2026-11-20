package root

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAccountStatementsCommandShape(t *testing.T) {
	cmd := newAccountStatementsCmd()
	if cmd.Use != "account-statements" {
		t.Fatalf("Use = %q, want account-statements", cmd.Use)
	}

	list := findSub(t, cmd, "list")
	for _, name := range []string{"account-id", "review-status", "content-sha256", "start-date", "end-date", "page", "per-page"} {
		if list.Flags().Lookup(name) == nil {
			t.Fatalf("expected flag %q on account-statements list", name)
		}
	}

	show := findSub(t, cmd, "show")
	if show.Args == nil {
		t.Fatal("account-statements show should require an id")
	}

	upload := findSub(t, cmd, "upload")
	for _, name := range []string{"file", "account-id", "apply"} {
		if upload.Flags().Lookup(name) == nil {
			t.Fatalf("expected flag %q on account-statements upload", name)
		}
	}
	if upload.Flags().Lookup("format") != nil {
		t.Fatal("upload must not shadow the global --format flag")
	}
}

func TestAccountStatementsCommandRegistered(t *testing.T) {
	root := New()
	for _, path := range [][]string{
		{"account-statements"},
		{"account-statements", "list"},
		{"account-statements", "show"},
		{"account-statements", "upload"},
	} {
		got, _, err := root.Find(path)
		if err != nil {
			t.Fatalf("path %v not registered: %v", path, err)
		}
		if got.Name() != path[len(path)-1] {
			t.Fatalf("path %v resolved to %q", path, got.Name())
		}
	}
}

func TestBuildAccountStatementListQuery_AllFilters(t *testing.T) {
	q, err := buildAccountStatementListQuery(accountStatementListOpts{
		AccountID:     "acc-1",
		ReviewStatus:  "linked",
		ContentSHA256: "ABCDEF",
		StartDate:     "2026-01-01",
		EndDate:       "2026-03-31",
		Page:          2,
		PerPage:       50,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := map[string]string{
		"account_id":     "acc-1",
		"review_status":  "linked",
		"content_sha256": "ABCDEF",
		"start_date":     "2026-01-01",
		"end_date":       "2026-03-31",
		"page":           "2",
		"per_page":       "50",
	}
	for k, v := range want {
		if got := q.Get(k); got != v {
			t.Fatalf("%s = %q, want %q", k, got, v)
		}
	}
}

func TestBuildAccountStatementListQuery_OmitsEmptyFilters(t *testing.T) {
	q, err := buildAccountStatementListQuery(accountStatementListOpts{Page: 1, PerPage: 25})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, k := range []string{"account_id", "review_status", "content_sha256", "start_date", "end_date"} {
		if q.Has(k) {
			t.Fatalf("%s should be omitted when empty", k)
		}
	}
}

func TestBuildAccountStatementListQuery_Rejections(t *testing.T) {
	cases := map[string]accountStatementListOpts{
		"bad review status": {ReviewStatus: "pending"},
		"bad start date":    {StartDate: "2026-13-01"},
		"bad end date":      {EndDate: "31/03/2026"},
	}
	for name, o := range cases {
		if _, err := buildAccountStatementListQuery(o); err == nil {
			t.Fatalf("%s: expected error", name)
		}
	}
}

func writeTempFile(t *testing.T, name string, content []byte) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, content, 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	return p
}

func TestBuildAccountStatementUpload_Valid(t *testing.T) {
	for ext, ct := range map[string]string{
		".pdf":  "application/pdf",
		".csv":  "text/csv",
		".XLSX": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
	} {
		path := writeTempFile(t, "statement"+ext, []byte("content"))
		u, err := buildAccountStatementUpload(accountStatementUploadOpts{File: path, AccountID: "acc-1"})
		if err != nil {
			t.Fatalf("%s: unexpected error: %v", ext, err)
		}
		if u.FilePath != path || mimeForUploadFile(u.FilePath) != ct || u.ByteSize != int64(len("content")) {
			t.Fatalf("%s: upload = %#v", ext, u)
		}
		if u.Fields["account_id"] != "acc-1" {
			t.Fatalf("%s: account_id = %q", ext, u.Fields["account_id"])
		}
	}
}

func TestBuildAccountStatementUpload_OmitsAccountIDWhenEmpty(t *testing.T) {
	path := writeTempFile(t, "s.pdf", []byte("x"))
	u, err := buildAccountStatementUpload(accountStatementUploadOpts{File: path})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, has := u.Fields["account_id"]; has {
		t.Fatal("account_id should be omitted when empty")
	}
}

func TestBuildAccountStatementUpload_Rejections(t *testing.T) {
	dir := t.TempDir()
	tooBig := filepath.Join(dir, "big.pdf")
	f, err := os.Create(tooBig)
	if err != nil {
		t.Fatal(err)
	}
	// Sparse file: one byte over the 25 MB server limit without writing it.
	if err := f.Truncate(accountStatementMaxBytes + 1); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	cases := map[string]string{
		"missing file":      "",
		"nonexistent file":  filepath.Join(dir, "nope.pdf"),
		"directory":         mustMkdir(t, filepath.Join(dir, "folder.pdf")),
		"bad extension":     writeTempFile(t, "statement.txt", []byte("x")),
		"no extension":      writeTempFile(t, "statement", []byte("x")),
		"ndjson not statem": writeTempFile(t, "statement.ndjson", []byte("x")),
		"empty file":        writeTempFile(t, "empty.csv", nil),
		"over 25MB":         tooBig,
	}
	for name, path := range cases {
		if _, err := buildAccountStatementUpload(accountStatementUploadOpts{File: path}); err == nil {
			t.Fatalf("%s: expected error", name)
		}
	}
}

func TestBuildAccountStatementUpload_AcceptsExactly25MB(t *testing.T) {
	p := filepath.Join(t.TempDir(), "max.csv")
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(accountStatementMaxBytes); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	if _, err := buildAccountStatementUpload(accountStatementUploadOpts{File: p}); err != nil {
		t.Fatalf("25 MB exactly should be accepted: %v", err)
	}
}

func mustMkdir(t *testing.T, p string) string {
	t.Helper()
	if err := os.Mkdir(p, 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestAccountStatementsUpload_DryRunByDefault(t *testing.T) {
	path := writeTempFile(t, "statement.pdf", []byte("%PDF-1.4"))
	cmd := newAccountStatementsCmd()
	cmd.SetArgs([]string{"upload", "--file", path, "--account-id", "acc-1"})

	out := captureStdout(t, func() {
		format = "json"
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute: %v", err)
		}
	})

	var env struct {
		Data struct {
			DryRun  bool `json:"dry_run"`
			Request struct {
				Method string         `json:"method"`
				Path   string         `json:"path"`
				Body   map[string]any `json:"body"`
			} `json:"request"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("unmarshal: %v\nout=%q", err, out)
	}
	if !env.Data.DryRun || env.Data.Request.Method != "POST" || env.Data.Request.Path != "/api/v1/account_statements" {
		t.Fatalf("unexpected dry-run envelope: %+v", env.Data)
	}
	body := env.Data.Request.Body
	if body["file"] != path || body["content_type"] != "application/pdf" || body["account_id"] != "acc-1" {
		t.Fatalf("dry-run body = %#v", body)
	}
	if body["byte_size"] != float64(len("%PDF-1.4")) {
		t.Fatalf("byte_size = %v", body["byte_size"])
	}
}

// TestPostAccountStatement_SendsMultipartWithExplicitContentType drives the
// apply path's HTTP call against a test server: the file part carries the
// parameter-free per-extension type, a hint AccountStatement's Marcel
// detection starts from before it checks ALLOWED_EXTENSION_CONTENT_TYPES.
func TestPostAccountStatement_SendsMultipartWithExplicitContentType(t *testing.T) {
	var gotPath, gotCT, gotFilename, gotAccount, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			t.Errorf("parse multipart: %v", err)
			return
		}
		gotAccount = r.FormValue("account_id")
		fh := r.MultipartForm.File["file"][0]
		gotCT = fh.Header.Get("Content-Type")
		gotFilename = fh.Filename
		f, _ := fh.Open()
		b, _ := io.ReadAll(f)
		gotBody = string(b)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"data":{"id":"st-1"},"meta":{"duplicate":false}}`))
	}))
	t.Cleanup(srv.Close)

	path := writeTempFile(t, "march.xlsx", []byte("PK\x03\x04"))
	u, err := buildAccountStatementUpload(accountStatementUploadOpts{File: path, AccountID: "acc-9"})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	var res map[string]any
	r, err := postAccountStatement(apiClientFor(t, srv.URL), u, &res)
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	if r.StatusCode() != http.StatusCreated {
		t.Fatalf("status = %d", r.StatusCode())
	}
	if gotPath != "/api/v1/account_statements" {
		t.Fatalf("path = %q", gotPath)
	}
	if gotCT != "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" {
		t.Fatalf("file part Content-Type = %q", gotCT)
	}
	if strings.Contains(gotCT, ";") {
		t.Fatalf("file part Content-Type must be parameter-free, got %q", gotCT)
	}
	if gotFilename != "march.xlsx" || gotBody != "PK\x03\x04" || gotAccount != "acc-9" {
		t.Fatalf("filename=%q body=%q account=%q", gotFilename, gotBody, gotAccount)
	}
	if res["meta"].(map[string]any)["duplicate"] != false {
		t.Fatalf("response not decoded: %#v", res)
	}
}
