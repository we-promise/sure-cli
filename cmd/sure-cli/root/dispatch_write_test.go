package root

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestDispatchWrite_DryRun_POST(t *testing.T) {
	out := captureStdout(t, func() {
		// Reset to json format so the envelope is parseable.
		format = "json"
		dispatchWrite(false, "POST", "/api/v1/chats", map[string]any{"title": "x"})
	})

	var env struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("unmarshal envelope: %v\nout=%q", err, out)
	}
	if env.Data["dry_run"] != true {
		t.Fatalf("expected dry_run=true, got %v", env.Data["dry_run"])
	}
	req, ok := env.Data["request"].(map[string]any)
	if !ok {
		t.Fatalf("request not map: %#v", env.Data["request"])
	}
	if req["method"] != "POST" || req["path"] != "/api/v1/chats" {
		t.Fatalf("method/path = %v / %v", req["method"], req["path"])
	}
	body, ok := req["body"].(map[string]any)
	if !ok || body["title"] != "x" {
		t.Fatalf("body = %#v", req["body"])
	}
}

func TestDispatchWrite_DryRun_DELETE_OmitsNilBody(t *testing.T) {
	out := captureStdout(t, func() {
		format = "json"
		dispatchWrite(false, "DELETE", "/api/v1/chats/abc", nil)
	})
	var env struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("unmarshal envelope: %v\nout=%q", err, out)
	}
	req, ok := env.Data["request"].(map[string]any)
	if !ok {
		t.Fatalf("request not map: %#v", env.Data["request"])
	}
	if req["method"] != "DELETE" || req["path"] != "/api/v1/chats/abc" {
		t.Fatalf("method/path = %v / %v", req["method"], req["path"])
	}
	if _, hasBody := req["body"]; hasBody {
		t.Fatalf("nil body should be omitted from dry-run output, got %#v", req)
	}
}

func TestDispatchWrite_ApplySendsEachMethod(t *testing.T) {
	// --apply must send the method the command names, to its path, with its
	// body, and print Sure's answer.
	type call struct {
		method, path string
		body         map[string]any
	}
	var got []call
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		got = append(got, call{r.Method, r.URL.Path, body})
		if r.Method == http.MethodDelete {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"t1","name":"renamed"}`))
	}))
	t.Cleanup(srv.Close)
	apiClientFor(t, srv.URL)

	for _, c := range []struct {
		args []string
		want call
		out  string
	}{
		{[]string{"create", "--name", "renamed", "--apply"}, call{"POST", "/api/v1/tags", map[string]any{"tag": map[string]any{"name": "renamed"}}}, `"renamed"`},
		{[]string{"update", "t1", "--name", "renamed", "--apply"}, call{"PATCH", "/api/v1/tags/t1", map[string]any{"tag": map[string]any{"name": "renamed"}}}, `"renamed"`},
		{[]string{"delete", "t1", "--apply"}, call{"DELETE", "/api/v1/tags/t1", nil}, `"deleted": true`},
	} {
		got = nil
		cmd := newTagsCmd()
		cmd.SetArgs(c.args)
		out := captureStdout(t, func() { _ = cmd.Execute() })
		if len(got) != 1 || got[0].method != c.want.method || got[0].path != c.want.path || fmt.Sprint(got[0].body) != fmt.Sprint(c.want.body) {
			t.Errorf("%v: sent %+v, want %+v", c.args, got, c.want)
		}
		if !strings.Contains(out, c.out) {
			t.Errorf("%v: printed %s, want it to contain %s", c.args, out, c.out)
		}
	}
}

// A delete Sure answers with 204 and no body (tags, chats) still prints data
// naming what was deleted, so the envelope has data or error like every other.
func TestPrintDelete_NoContentNamesTheDeletedID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(srv.Close)
	apiClientFor(t, srv.URL)

	out := captureStdout(t, func() { printDelete("/api/v1/tags/" + url.PathEscape("a/b c")) })
	var env struct {
		Data map[string]any `json:"data"`
		Meta struct {
			Status int `json:"status"`
		} `json:"meta"`
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if fmt.Sprint(env.Data) != fmt.Sprint(map[string]any{"id": "a/b c", "deleted": true}) || env.Meta.Status != http.StatusNoContent {
		t.Fatalf("printed %s, want data {id: \"a/b c\", deleted: true} and meta.status 204", out)
	}
}
