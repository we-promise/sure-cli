package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/spf13/viper"
	"github.com/we-promise/sure-cli/internal/config"
	errs "github.com/we-promise/sure-cli/internal/errors"
)

// testClient returns an api_key Client against srvURL with retry waits shrunk
// so retry tests stay fast without changing production backoff.
func testClient(t *testing.T, srvURL string) *Client {
	t.Helper()
	viper.Reset()
	viper.Set("auth.mode", "api_key")
	viper.Set("auth.api_key", "key")
	_ = config.Init("/tmp/does-not-exist.yaml")
	viper.Set("api_url", srvURL)
	c := New()
	c.http.SetRetryWaitTime(time.Millisecond).SetRetryMaxWaitTime(50 * time.Millisecond)
	return c
}

// countingServer answers every request with status, headers and body, and
// counts hits per HTTP method.
func countingServer(t *testing.T, status int, headers map[string]string, body string) (*httptest.Server, map[string]int) {
	t.Helper()
	hits := map[string]int{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits[r.Method]++
		for k, v := range headers {
			w.Header().Set(k, v)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv, hits
}

func TestClient_Retries5xxOnlyForIdempotentMethods(t *testing.T) {
	srv, hits := countingServer(t, http.StatusInternalServerError, nil, `{"error":"internal_server_error"}`)
	c := testClient(t, srv.URL)
	var out any

	_, _ = c.Get("/x", &out)
	_, _ = c.Put("/x", map[string]any{}, &out)
	_, _ = c.Delete("/x", &out)
	_, _ = c.Post("/x", map[string]any{}, &out)
	_, _ = c.Patch("/x", map[string]any{}, &out)

	tmp := t.TempDir() + "/f.csv"
	if err := os.WriteFile(tmp, []byte("a,b\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, _ = c.PostMultipart("/x", nil, "file", tmp, "text/csv", &out)

	// 1 attempt + 2 retries for idempotent methods.
	for _, m := range []string{http.MethodGet, http.MethodPut} {
		if hits[m] != 3 {
			t.Errorf("%s hits = %d, want 3 (retried)", m, hits[m])
		}
	}
	// A proxy can lose the answer to a DELETE Sure already committed; the
	// retry would then report not_found for a delete that succeeded.
	if hits[http.MethodDelete] != 1 {
		t.Errorf("DELETE hits = %d, want 1 (never retried)", hits[http.MethodDelete])
	}
	// Sure may persist before answering 5xx (e.g. imports publish_failed
	// returns 500 with the import_id); a retry would duplicate the write.
	if hits[http.MethodPost] != 2 { // Post + PostMultipart, once each
		t.Errorf("POST hits = %d, want 2 (never retried)", hits[http.MethodPost])
	}
	if hits[http.MethodPatch] != 1 {
		t.Errorf("PATCH hits = %d, want 1 (never retried)", hits[http.MethodPatch])
	}
}

func TestClient_RetriesNetworkErrorsOnlyForIdempotentMethods(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := srv.URL
	srv.Close() // connection refused from now on

	c := testClient(t, url)
	// resty fires retry hooks after every attempt the retry condition
	// accepts, including the last one, so hooks == attempts when retrying.
	hooks := 0
	c.http.AddRetryHook(func(*resty.Response, error) { hooks++ })

	var out any
	if _, err := c.Post("/x", map[string]any{}, &out); err == nil {
		t.Fatal("expected network error")
	}
	if hooks != 0 {
		t.Fatalf("POST retry hooks = %d on network error, want 0 (never retried)", hooks)
	}
	if _, err := c.Get("/x", &out); err == nil {
		t.Fatal("expected network error")
	}
	if hooks != 3 {
		t.Fatalf("GET retry hooks = %d, want 3 (1 attempt + 2 retries)", hooks)
	}
}

func TestClient_RetriesRateLimitedIdempotentRequestsHonoringRetryAfter(t *testing.T) {
	srv, hits := countingServer(t, http.StatusTooManyRequests, map[string]string{"Retry-After": "0"}, `{"error":"rate_limit_exceeded"}`)
	c := testClient(t, srv.URL)
	var out any

	_, _ = c.Get("/x", &out)
	_, _ = c.Post("/x", map[string]any{}, &out)

	if hits[http.MethodGet] != 3 {
		t.Errorf("GET hits = %d, want 3", hits[http.MethodGet])
	}
	if hits[http.MethodPost] != 1 {
		t.Errorf("POST hits = %d, want 1", hits[http.MethodPost])
	}
}

func TestClient_DoesNotRetryRateLimitBeyondMaxWait(t *testing.T) {
	// Sure sets Retry-After to the seconds left in the rate-limit window (up
	// to an hour). Retrying after the capped max wait would only burn the
	// remaining attempts on guaranteed 429s.
	for _, h := range []map[string]string{{"Retry-After": "3600"}, nil} {
		srv, hits := countingServer(t, http.StatusTooManyRequests, h, `{"error":"rate_limit_exceeded"}`)
		c := testClient(t, srv.URL)
		var out any

		_, _ = c.Get("/x", &out)
		if hits[http.MethodGet] != 1 {
			t.Errorf("headers %v: GET hits = %d, want 1", h, hits[http.MethodGet])
		}
	}
}

func TestRetryAfter(t *testing.T) {
	cases := []struct {
		header string
		want   time.Duration
		ok     bool
	}{
		{"", 0, false},
		{"0", 0, true},
		{"7", 7 * time.Second, true},
		{" 12 ", 12 * time.Second, true},
		{"-1", 0, false},
		{"soon", 0, false},
	}
	for _, c := range cases {
		h := http.Header{}
		if c.header != "" {
			h.Set("Retry-After", c.header)
		}
		got, ok := retryAfter(h)
		if got != c.want || ok != c.ok {
			t.Errorf("retryAfter(%q) = %v,%v want %v,%v", c.header, got, ok, c.want, c.ok)
		}
	}
}

func TestShouldRetry_RateLimitAtMaxWaitBoundary(t *testing.T) {
	maxWait := 5 * time.Second
	for header, want := range map[string]bool{"4": true, "5": true, "6": false} {
		r := &resty.Response{
			Request:     &resty.Request{Method: http.MethodGet},
			RawResponse: &http.Response{StatusCode: http.StatusTooManyRequests, Header: http.Header{"Retry-After": {header}}},
		}
		if got := shouldRetry(r, nil, maxWait); got != want {
			t.Errorf("Retry-After %s with max wait %v: retry = %v, want %v", header, maxWait, got, want)
		}
	}
}

func TestClient_RetryThatCannotFitTheTimeoutReturnsTheLastResponse(t *testing.T) {
	srv, hits := countingServer(t, http.StatusServiceUnavailable, nil, `{"error":"maintenance","message":"Back soon"}`)
	c := testClient(t, srv.URL)
	c.http.SetRetryWaitTime(200 * time.Millisecond).SetRetryMaxWaitTime(time.Second)
	c.timeout = 300 * time.Millisecond

	var out any
	ce := CheckResponse(c.Get("/x", &out))
	// Attempt 1, a 200ms wait, attempt 2; the next 400ms wait would outlast
	// the timeout, so Sure's 503 is reported instead of a timeout.
	if ce == nil || ce.Code != errs.CodeServerError || ce.Message != "Back soon" {
		t.Fatalf("got %v, want Sure's 503", ce)
	}
	if hits[http.MethodGet] != 2 {
		t.Fatalf("GET hits = %d, want 2", hits[http.MethodGet])
	}
}

func TestRetryWait(t *testing.T) {
	base, ceiling := time.Second, 5*time.Second
	for _, tc := range []struct {
		attempt    int
		retryAfter string
		want       time.Duration
	}{
		{1, "", time.Second},
		{2, "", 2 * time.Second},
		{3, "", 4 * time.Second},
		{4, "", 5 * time.Second},
		{1, "3", 3 * time.Second},
		{1, "0", time.Second},
	} {
		h := http.Header{}
		if tc.retryAfter != "" {
			h.Set("Retry-After", tc.retryAfter)
		}
		r := &resty.Response{Request: &resty.Request{Attempt: tc.attempt}, RawResponse: &http.Response{Header: h}}
		if got := retryWait(r, base, ceiling); got != tc.want {
			t.Errorf("attempt %d Retry-After %q: wait = %v, want %v", tc.attempt, tc.retryAfter, got, tc.want)
		}
	}
}
