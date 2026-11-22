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

// Sure renders the merchants and tags indexes unpaginated and ignores
// page/per_page, so the CLI does not offer flags that would do nothing.
func TestUnpaginatedLists_NoPagingFlags(t *testing.T) {
	for _, group := range []string{"merchants", "tags"} {
		list := findSub(t, findSub(t, New(), group), "list")
		for _, name := range []string{"page", "per-page"} {
			if list.Flags().Lookup(name) != nil {
				t.Errorf("%s list must not offer --%s: Sure ignores it", group, name)
			}
		}
	}
}

// Sure serves pages of 1 to 100 rows. Outside that range some endpoints
// serve 100 and others 25, so the CLI refuses the value instead.
func TestPagingFlagsOutOfRangeFailValidation(t *testing.T) {
	queries := make(chan string, 16)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case queries <- r.URL.RawQuery:
		default:
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"categories":[]}`))
	}))
	t.Cleanup(srv.Close)
	cfg := apiKeyConfig(t, t.TempDir(), srv.URL)
	for _, args := range [][]string{
		{"categories", "list", "--per-page", "101"}, {"categories", "list", "--per-page", "0"},
		{"categories", "list", "--per-page", "-1"}, {"categories", "list", "--page", "0"},
		// Chats have a fixed page size, but --page follows the same rule.
		{"chats", "list", "--page", "0"}, {"chats", "show", "c1", "--page", "-1"},
	} {
		flag := args[len(args)-2]
		if ce := cliError(t, append([]string{"--config", cfg}, args...)...); ce == nil || ce.Code != "validation_failed" || !strings.Contains(ce.Message, flag) {
			t.Errorf("%v: error = %v, want validation_failed naming %s", args, ce, flag)
		}
	}
	if len(queries) > 0 {
		t.Fatalf("sent %q for a refused paging flag", <-queries)
	}
	for flags, want := range map[string]string{"--per-page 100": "page=1&per_page=100", "--per-page 1 --page 3": "page=3&per_page=1"} {
		args := append([]string{"--config", cfg, "categories", "list"}, strings.Fields(flags)...)
		if ce := cliError(t, args...); ce != nil {
			t.Fatalf("%s: %v", flags, ce)
		}
		if got := <-queries; got != want {
			t.Errorf("%s: sent ?%s, want ?%s", flags, got, want)
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
	if err := validateMerchantsImportFile(writeTempFile(t, "merchants.txt", []byte("name\nRoaster\n"))); err == nil {
		t.Fatal("expected err for a file that is not .csv")
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
	r, err := upload{FilePath: path}.send(apiClientFor(t, srv.URL), merchantsPath, &res)
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
