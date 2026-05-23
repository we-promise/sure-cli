package root

import (
	"encoding/json"
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
