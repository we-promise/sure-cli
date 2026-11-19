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

// Sure renders the merchants index as a bare, unpaginated array and ignores
// page/per_page, so the CLI does not offer flags that would do nothing.
func TestMerchantsList_NoPagingFlags(t *testing.T) {
	list := findSub(t, findSub(t, New(), "merchants"), "list")
	for _, name := range []string{"page", "per-page"} {
		if list.Flags().Lookup(name) != nil {
			t.Fatalf("merchants list must not offer --%s: Sure ignores it", name)
		}
	}
}

func TestRuleRunsList_ExecutionTypeHelpNamesImmediate(t *testing.T) {
	list := findSub(t, findSub(t, New(), "rule-runs"), "list")
	usage := list.Flags().Lookup("execution-type").Usage
	if !strings.Contains(usage, "manual|scheduled|immediate") {
		t.Fatalf("execution-type help = %q, want manual|scheduled|immediate", usage)
	}
}

func writeMerchantsCSV(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "merchants.csv")
	if err := os.WriteFile(path, []byte("name,color,website_url\nRoaster,#112233,https://roaster.example\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestValidateMerchantsImportFile(t *testing.T) {
	if err := validateMerchantsImportFile(""); err == nil {
		t.Fatal("expected err for missing --file")
	}
	if err := validateMerchantsImportFile(t.TempDir()); err == nil {
		t.Fatal("expected err for a directory")
	}
	if err := validateMerchantsImportFile(filepath.Join(t.TempDir(), "missing.csv")); err == nil {
		t.Fatal("expected err for a missing file")
	}
	if err := validateMerchantsImportFile(writeMerchantsCSV(t)); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
}

func TestMerchantsImport_DryRun(t *testing.T) {
	path := writeMerchantsCSV(t)
	out := captureStdout(t, func() {
		format = "json"
		root := New()
		root.SetArgs([]string{"merchants", "import", "--file", path})
		if err := root.Execute(); err != nil {
			t.Errorf("execute: %v", err)
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
	req := env.Data.Request
	if !env.Data.DryRun || req.Method != "POST" || req.Path != "/api/v1/merchants" || req.Body["file"] != path {
		t.Fatalf("unexpected dry-run: %+v", env.Data)
	}
}

// POST /api/v1/merchants reads a multipart `file` part and checks its
// content type against Import::ALLOWED_CSV_MIME_TYPES (exact match).
func TestPostMerchantsImport_SendsCSVMultipart(t *testing.T) {
	path := writeMerchantsCSV(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/merchants" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			t.Errorf("file part: %v", err)
		} else {
			defer file.Close()
			if ct := header.Header.Get("Content-Type"); ct != "text/csv" {
				t.Errorf("file part Content-Type = %q, want text/csv", ct)
			}
			body, _ := io.ReadAll(file)
			if !strings.HasPrefix(string(body), "name,color,website_url\n") {
				t.Errorf("file part body = %q", body)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"imported":1,"skipped":0,"merchants":[{"id":"m1","name":"Roaster"}]}`))
	}))
	t.Cleanup(srv.Close)

	var res map[string]any
	r, err := postMerchantsImport(apiClientFor(t, srv.URL), path, &res)
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	if r.StatusCode() != http.StatusCreated || res["imported"] != float64(1) {
		t.Fatalf("status=%d res=%v", r.StatusCode(), res)
	}
}

func TestBuildTagCreatePayload_RequiresName(t *testing.T) {
	if _, err := buildTagCreatePayload(tagWriteOpts{}); err == nil {
		t.Fatal("expected missing name error")
	}
	payload, err := buildTagCreatePayload(tagWriteOpts{Name: "Travel", Color: "#3b82f6"})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	tag, ok := payload["tag"].(map[string]any)
	if !ok {
		t.Fatalf("expected payload[\"tag\"] to be map[string]any, got %T: %#v", payload["tag"], payload)
	}
	if tag["name"] != "Travel" || tag["color"] != "#3b82f6" {
		t.Fatalf("unexpected tag payload: %#v", tag)
	}
}

func TestBuildTagCreatePayload_ColorOptional(t *testing.T) {
	payload, err := buildTagCreatePayload(tagWriteOpts{Name: "Travel"})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	tag := payload["tag"].(map[string]any)
	if _, has := tag["color"]; has {
		t.Fatalf("color should be omitted when empty, got %#v", tag)
	}
}

func TestBuildTagUpdatePayload_NeedsAtLeastOneField(t *testing.T) {
	if _, err := buildTagUpdatePayload(tagWriteOpts{}); err == nil {
		t.Fatal("expected error when no fields provided")
	}
}

func TestBuildTagUpdatePayload_PartialOK(t *testing.T) {
	payload, err := buildTagUpdatePayload(tagWriteOpts{Color: "#ff0000"})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	tag := payload["tag"].(map[string]any)
	if tag["color"] != "#ff0000" {
		t.Fatalf("color = %v", tag["color"])
	}
	if _, has := tag["name"]; has {
		t.Fatalf("name should be omitted, got %#v", tag)
	}
}

func TestReferenceCommandsRegistered(t *testing.T) {
	root := New()
	// cobra's Find returns the nearest matching ancestor when a leaf is
	// missing, so compare resolved Name to the expected leaf.
	cases := []struct {
		args []string
		want string
	}{
		{[]string{"categories", "list"}, "list"},
		{[]string{"categories", "show"}, "show"},
		{[]string{"categories", "create"}, "create"},
		{[]string{"merchants", "list"}, "list"},
		{[]string{"merchants", "show"}, "show"},
		{[]string{"merchants", "import"}, "import"},
		{[]string{"tags", "create"}, "create"},
		{[]string{"tags", "update"}, "update"},
		{[]string{"tags", "delete"}, "delete"},
		{[]string{"rules", "list"}, "list"},
		{[]string{"rule-runs", "show"}, "show"},
	}
	for _, c := range cases {
		got, _, err := root.Find(c.args)
		if err != nil {
			t.Fatalf("path %v not registered: %v", c.args, err)
		}
		if got.Name() != c.want {
			t.Fatalf("path %v resolved to %q, want %q", c.args, got.Name(), c.want)
		}
	}
}
