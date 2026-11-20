package root

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// proposeServer serves, for a family in UTC, two merchants whose rows are
// mostly Food: each yields a proposal to categorise its 2 uncategorised rows,
// 4 updates in all. failPut answers the n-th PUT (1-based) with a status, or
// 0 to accept it. It returns its URL and the ids PUT so far.
func proposeServer(t *testing.T, failPut func(n int) int) (string, func() []string) {
	t.Helper()
	var rows []string
	for _, merchant := range []string{"Coffee", "Bakery"} {
		for i := range 10 {
			cat := `,"category":{"id":"c1","name":"Food"}`
			if i >= 8 {
				cat = ""
			}
			rows = append(rows, fmt.Sprintf(`{"id":"%s-%d","date":%q,"amount":"1.0","amount_decimal":"1.0","currency":"EUR","name":%q,`+
				`"classification":"expense","kind":"standard","account":{"id":"a1","name":"Main"}%s}`,
				strings.ToLower(merchant), i, time.Now().UTC().Format(time.DateOnly), merchant, cat))
		}
	}
	var mu sync.Mutex
	var put []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/v1/family_settings":
			fmt.Fprint(w, `{"timezone":"UTC"}`)
		case r.URL.Path == "/api/v1/accounts":
			fmt.Fprint(w, `{"accounts":[{"id":"a1","exclude_from_reports":false}],"pagination":{"page":1,"total_pages":1}}`)
		case r.URL.Path == "/api/v1/transactions":
			fmt.Fprintf(w, `{"transactions":[%s],"pagination":{"page":1,"total_pages":1}}`, strings.Join(rows, ","))
		case r.Method == http.MethodPut && strings.HasPrefix(r.URL.Path, "/api/v1/transactions/"):
			mu.Lock()
			put = append(put, strings.TrimPrefix(r.URL.Path, "/api/v1/transactions/"))
			n := len(put)
			mu.Unlock()
			switch status := failPut(n); status {
			case 0:
				fmt.Fprint(w, `{"id":"x"}`)
			case http.StatusTooManyRequests:
				w.Header().Set("Retry-After", "3600")
				w.WriteHeader(status)
				fmt.Fprint(w, `{"error":"rate_limit_exceeded","message":"slow down"}`)
			default:
				w.WriteHeader(status)
				fmt.Fprint(w, `{"error":"not_found","message":"gone"}`)
			}
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	return srv.URL, func() []string {
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), put...)
	}
}

// applyResult runs `propose rules --apply` in a child process and returns
// its exit error and envelope.
func applyResult(t *testing.T, url string) (error, map[string]any) {
	t.Helper()
	child, stdout := startCLI(t, "--config", apiKeyConfig(t, t.TempDir(), url), "propose", "rules", "--apply")
	err := child.Wait()
	var env map[string]any
	if jerr := json.Unmarshal(stdout.Bytes(), &env); jerr != nil {
		t.Fatalf("stdout is not an envelope: %v\n%s", jerr, stdout)
	}
	return err, env
}

func TestProposeApply_StopsAtTheRateLimit(t *testing.T) {
	url, puts := proposeServer(t, func(n int) int {
		if n == 2 {
			return http.StatusTooManyRequests
		}
		return 0
	})
	err, env := applyResult(t, url)
	if err == nil {
		t.Fatalf("exit 0 after a rate limit: %v", env)
	}
	if got := puts(); len(got) != 2 {
		t.Fatalf("sent %d updates (%v), want none after the 429", len(got), got)
	}
	e, _ := env["error"].(map[string]any)
	if e == nil || e["code"] != "apply_incomplete" {
		t.Fatalf("envelope = %v, want error apply_incomplete", env)
	}
	d, _ := e["details"].(map[string]any)
	if d["applied_count"] != float64(1) || d["error_count"] != float64(1) || d["skipped_count"] != float64(2) {
		t.Fatalf("details = %v, want 1 applied, 1 failed, 2 skipped", d)
	}
	failed := d["errors"].([]any)[0].(map[string]any)
	if failed["code"] != "rate_limited" {
		t.Fatalf("error = %v, want rate_limited", failed)
	}
	for _, s := range d["skipped"].([]any) {
		if s.(map[string]any)["reason"] != "rate_limited" || s.(map[string]any)["tx_id"] == nil {
			t.Fatalf("skipped = %v, want each remaining update skipped as rate_limited", d["skipped"])
		}
	}
}

func TestProposeApply_AnyFailureExitsNonZero(t *testing.T) {
	url, puts := proposeServer(t, func(n int) int {
		if n == 1 {
			return http.StatusNotFound
		}
		return 0
	})
	err, env := applyResult(t, url)
	if err == nil {
		t.Fatalf("exit 0 with a failed update: %v", env)
	}
	if got := puts(); len(got) != 4 {
		t.Fatalf("sent %d updates, want all 4: only a rate limit stops the run", len(got))
	}
	d := env["error"].(map[string]any)["details"].(map[string]any)
	if d["applied_count"] != float64(3) || d["error_count"] != float64(1) || d["errors"].([]any)[0].(map[string]any)["code"] != "not_found" {
		t.Fatalf("details = %v, want 3 applied and 1 not_found", d)
	}
}

func TestProposeApply_AllAppliedExitsZero(t *testing.T) {
	url, _ := proposeServer(t, func(int) int { return 0 })
	err, env := applyResult(t, url)
	if err != nil {
		t.Fatalf("exit %v: %v", err, env)
	}
	if d := env["data"].(map[string]any); d["applied_count"] != float64(4) || d["error_count"] != float64(0) {
		t.Fatalf("data = %v, want 4 applied", d)
	}
}
