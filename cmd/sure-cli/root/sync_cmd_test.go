package root

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSync_IsADryRunUnlessApplied(t *testing.T) {
	posts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/sync" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		posts++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"id":"s1","status":"pending"}`))
	}))
	t.Cleanup(srv.Close)
	apiClientFor(t, srv.URL)

	cmd := newSyncCmd()
	cmd.SetArgs([]string{})
	out := captureStdout(t, func() { _ = cmd.Execute() })
	if posts != 0 || !strings.Contains(out, `"dry_run": true`) || !strings.Contains(out, "/api/v1/sync") {
		t.Fatalf("dry run sent %d requests, printed %s", posts, out)
	}

	cmd = newSyncCmd()
	cmd.SetArgs([]string{"--apply"})
	out = captureStdout(t, func() { _ = cmd.Execute() })
	if posts != 1 || !strings.Contains(out, `"s1"`) {
		t.Fatalf("--apply sent %d requests, printed %s", posts, out)
	}
}
