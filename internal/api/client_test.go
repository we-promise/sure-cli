package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/spf13/viper"

	"github.com/we-promise/sure-cli/internal/config"
	errs "github.com/we-promise/sure-cli/internal/errors"
)

func TestClient_BearerAuthHeader(t *testing.T) {
	viper.Reset()
	viper.Set("api_url", "http://example.invalid")
	viper.Set("auth.mode", "bearer")
	viper.Set("auth.token", "tok_123")

	_ = config.Init("/tmp/does-not-exist.yaml") // sets defaults

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer tok_123" {
			t.Fatalf("expected Authorization header, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	viper.Set("api_url", srv.URL)
	c := New()
	var out any
	_, err := c.Get("/api/v1/usage", &out)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
}

func TestClient_AutoRefresh(t *testing.T) {
	viper.Reset()
	cfg := t.TempDir() + "/config.yaml"
	_ = config.Init(cfg)

	viper.Set("auth.mode", "bearer")
	viper.Set("auth.token", "tok_old")
	viper.Set("auth.refresh_token", "ref_123")
	viper.Set("auth.token_expires_at", time.Now().Add(-1*time.Minute).UTC().Format(time.RFC3339))
	viper.Set("auth.device.device_id", "sure-cli")
	viper.Set("auth.device.device_name", "sure-cli")
	viper.Set("auth.device.device_type", "web")
	viper.Set("auth.device.os_version", "test")
	viper.Set("auth.device.app_version", "test")

	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/api/v1/auth/refresh":
			_, _ = w.Write([]byte(`{"access_token":"tok_new","refresh_token":"ref_new","token_type":"Bearer","expires_in":3600,"created_at":123}`))
		case "/api/v1/usage":
			if got := r.Header.Get("Authorization"); got != "Bearer tok_new" {
				t.Fatalf("expected refreshed Authorization header, got %q", got)
			}
			w.WriteHeader(200)
			_, _ = w.Write([]byte(`{"ok":true}`))
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()

	viper.Set("api_url", srv.URL)
	c := New()
	var out any
	_, err := c.Get("/api/v1/usage", &out)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if calls < 2 {
		t.Fatalf("expected refresh + request, got %d calls", calls)
	}
}

func TestClient_APIKeyHeader(t *testing.T) {
	viper.Reset()
	viper.Set("api_url", "http://example.invalid")
	viper.Set("auth.mode", "api_key")
	viper.Set("auth.api_key", "key_456")
	_ = config.Init("/tmp/does-not-exist.yaml")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-Api-Key"); got != "key_456" {
			t.Fatalf("expected X-Api-Key header, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	viper.Set("api_url", srv.URL)
	c := New()
	var out any
	_, err := c.Get("/api/v1/usage", &out)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
}

func TestClient_PostMultipart(t *testing.T) {
	viper.Reset()
	viper.Set("auth.mode", "api_key")
	viper.Set("auth.api_key", "key_456")
	_ = config.Init("/tmp/does-not-exist.yaml")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify it's a multipart request
		contentType := r.Header.Get("Content-Type")
		if !strings.Contains(contentType, "multipart/form-data") {
			t.Fatalf("expected multipart/form-data, got %q", contentType)
		}

		// Parse multipart form
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			t.Fatalf("parse multipart: %v", err)
		}

		// Verify form fields
		if got := r.FormValue("format"); got != "csv" {
			t.Fatalf("expected format=csv, got %q", got)
		}
		if got := r.FormValue("source"); got != "test" {
			t.Fatalf("expected source=test, got %q", got)
		}

		// Verify file
		file, header, err := r.FormFile("file")
		if err != nil {
			t.Fatalf("get form file: %v", err)
		}
		defer file.Close()

		if header.Filename == "" {
			t.Fatal("expected filename to be set")
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"id":"imp_123","status":"pending"}`))
	}))
	defer srv.Close()

	viper.Set("api_url", srv.URL)

	// Create temp file
	tmpFile := t.TempDir() + "/test.csv"
	if err := os.WriteFile(tmpFile, []byte("col1,col2\nval1,val2"), 0644); err != nil {
		t.Fatalf("create temp file: %v", err)
	}

	c := New()
	fields := map[string]string{
		"format": "csv",
		"source": "test",
	}

	var out map[string]any
	_, err := c.PostMultipart("/api/v1/imports", fields, "file", tmpFile, "", &out)
	if err != nil {
		t.Fatalf("PostMultipart failed: %v", err)
	}

	if out["id"] != "imp_123" {
		t.Errorf("expected id=imp_123, got %v", out["id"])
	}
}

func TestClient_PostMultipart_MismatchedArgs(t *testing.T) {
	viper.Reset()
	viper.Set("auth.mode", "api_key")
	viper.Set("auth.api_key", "key_456")
	viper.Set("api_url", "http://example.invalid")
	_ = config.Init("/tmp/does-not-exist.yaml")

	c := New()

	// fileField set but filePath empty
	_, err := c.PostMultipart("/api/v1/imports", nil, "file", "", "", nil)
	if err == nil {
		t.Fatal("expected error for mismatched file arguments")
	}

	// filePath set but fileField empty
	_, err = c.PostMultipart("/api/v1/imports", nil, "", "/some/path", "", nil)
	if err == nil {
		t.Fatal("expected error for mismatched file arguments")
	}
}

// TestClient_PostMultipart_ExplicitContentType locks in the fix for the
// imports preflight bug: resty's default SetFile sniffs CSV content as
// "text/plain; charset=utf-8", and Sure's exact-match content-type
// allow-list (Import::ALLOWED_CSV_MIME_TYPES) rejects the charset suffix.
// Passing an explicit content-type forces the multipart part header so the
// upstream check sees a parameter-free value.
func TestClient_PostMultipart_ExplicitContentType(t *testing.T) {
	viper.Reset()
	viper.Set("auth.mode", "api_key")
	viper.Set("auth.api_key", "key_456")
	_ = config.Init("/tmp/does-not-exist.yaml")

	var capturedCT string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			t.Fatalf("parse multipart: %v", err)
		}
		fhs := r.MultipartForm.File["file"]
		if len(fhs) != 1 {
			t.Fatalf("expected one file part, got %d", len(fhs))
		}
		capturedCT = fhs[0].Header.Get("Content-Type")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	viper.Set("api_url", srv.URL)
	tmpFile := t.TempDir() + "/test.csv"
	if err := os.WriteFile(tmpFile, []byte("col1,col2\nval1,val2"), 0644); err != nil {
		t.Fatalf("create temp file: %v", err)
	}

	c := New()
	if _, err := c.PostMultipart("/api/v1/imports/preflight", nil, "file", tmpFile, "text/csv", nil); err != nil {
		t.Fatalf("PostMultipart failed: %v", err)
	}
	if capturedCT != "text/csv" {
		t.Fatalf("expected Content-Type=text/csv (parameter-free, to satisfy Sure's exact-match allow-list), got %q", capturedCT)
	}
}

// TestClient_PostMultipart_DefaultContentType confirms that passing "" for
// the content type preserves the pre-fix behavior (resty detects from content),
// so existing call sites that don't need the explicit override remain unchanged.
func TestClient_PostMultipart_DefaultContentType(t *testing.T) {
	viper.Reset()
	viper.Set("auth.mode", "api_key")
	viper.Set("auth.api_key", "key_456")
	_ = config.Init("/tmp/does-not-exist.yaml")

	var capturedCT string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseMultipartForm(10 << 20)
		fhs := r.MultipartForm.File["file"]
		if len(fhs) == 1 {
			capturedCT = fhs[0].Header.Get("Content-Type")
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	viper.Set("api_url", srv.URL)
	tmpFile := t.TempDir() + "/test.csv"
	if err := os.WriteFile(tmpFile, []byte("col1,col2\nval1,val2"), 0644); err != nil {
		t.Fatalf("create temp file: %v", err)
	}

	c := New()
	if _, err := c.PostMultipart("/api/v1/imports/preflight", nil, "file", tmpFile, "", nil); err != nil {
		t.Fatalf("PostMultipart failed: %v", err)
	}
	// The current resty default for an ASCII CSV is "text/plain; charset=utf-8".
	// Lock the non-empty default so future resty upgrades that change this
	// signal a deliberate behavior change here.
	if capturedCT == "" {
		t.Fatalf("expected non-empty default Content-Type from resty's auto-detection, got empty")
	}
}

func TestClient_Patch(t *testing.T) {
	viper.Reset()
	viper.Set("auth.mode", "api_key")
	viper.Set("auth.api_key", "key_456")
	_ = config.Init("/tmp/does-not-exist.yaml")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	viper.Set("api_url", srv.URL)
	c := New()
	var out map[string]any
	_, err := c.Patch("/api/v1/transactions/tx_123", map[string]any{"transaction": map[string]any{"name": "x"}}, &out)
	if err != nil {
		t.Fatalf("Patch failed: %v", err)
	}
	if out["ok"] != true {
		t.Fatalf("expected ok response, got %#v", out)
	}
}

func TestClient_Unusable2xxBodyIsInvalidResponse(t *testing.T) {
	// A captive portal or SSO proxy answering 200 with HTML, a proxy cutting
	// the JSON short, a 201 with no body: none of these is a success, and
	// the status tells an agent whether a write was committed.
	cases := []struct {
		name, method, contentType, body string
		status                          int
	}{
		{"html GET", http.MethodGet, "text/html; charset=utf-8", "<html>Please log in to the proxy</html>", http.StatusOK},
		{"html POST", http.MethodPost, "text/html", "<html>Please log in</html>", http.StatusOK},
		{"truncated JSON GET", http.MethodGet, "application/json", `{"accounts":[{"id":"1"`, http.StatusOK},
		{"empty created POST", http.MethodPost, "application/json", "", http.StatusCreated},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			hits := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hits++
				w.Header().Set("Content-Type", tc.contentType)
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()
			c := testClient(t, srv.URL)

			var out any
			var r *resty.Response
			var err error
			if tc.method == http.MethodGet {
				r, err = c.Get("/x", &out)
			} else {
				r, err = c.Post("/x", map[string]any{}, &out)
			}
			ce := CheckResponse(r, err)
			if ce == nil || ce.Code != errs.CodeInvalidResponse {
				t.Fatalf("got %v, want invalid_response", ce)
			}
			if ce.Details["status"] != tc.status {
				t.Fatalf("details.status = %v, want %d", ce.Details["status"], tc.status)
			}
			if hits != 1 {
				t.Fatalf("hits = %d, want 1 (a decode failure is never retried)", hits)
			}
		})
	}
}

func TestClient_NoContentLeavesResultUntouched(t *testing.T) {
	srv, _ := countingServer(t, http.StatusNoContent, nil, "")
	c := testClient(t, srv.URL)
	var out any
	if ce := CheckResponse(c.Delete("/api/v1/tags/1", &out)); ce != nil || out != nil {
		t.Fatalf("ce=%v out=%v", ce, out)
	}
}

func TestClient_OversizedBodyIsInvalidResponseNotRetried(t *testing.T) {
	srv, hits := countingServer(t, http.StatusOK, nil, `{"data":"`+strings.Repeat("x", 64)+`"}`)
	c := testClient(t, srv.URL)
	c.http.SetResponseBodyLimit(16)
	var out any
	ce := CheckResponse(c.Get("/x", &out))
	if ce == nil || ce.Code != errs.CodeInvalidResponse {
		t.Fatalf("got %v, want invalid_response", ce)
	}
	if hits[http.MethodGet] != 1 {
		t.Fatalf("GET hits = %d, want 1", hits[http.MethodGet])
	}
}

func TestClient_TimeoutBoundsTheCall(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(2 * time.Second):
		}
	}))
	defer srv.Close()
	viper.Reset()
	_ = config.Init(filepath.Join(t.TempDir(), "none.yaml"))
	viper.Set("api_url", srv.URL)
	viper.Set("auth.mode", "api_key")
	viper.Set("timeout", "100ms")
	c := New()

	start := time.Now()
	var out any
	ce := CheckResponse(c.Get("/x", &out))
	if ce == nil || ce.Code != errs.CodeTimeout {
		t.Fatalf("got %v, want timeout", ce)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("call took %v; the timeout must bound retries too", elapsed)
	}
}

// expiredBearerConfig writes a bearer config whose access token needs a
// refresh and points the client at srvURL.
func expiredBearerConfig(t *testing.T, dir, srvURL string) string {
	t.Helper()
	cfg := filepath.Join(dir, "config.yaml")
	body := fmt.Sprintf("api_url: %s\nauth:\n  mode: bearer\n  token: at-old\n  refresh_token: rt-old\n  token_expires_at: %q\n",
		srvURL, time.Now().Add(-time.Minute).UTC().Format(time.RFC3339))
	if err := os.WriteFile(cfg, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	viper.Reset()
	if err := config.Init(cfg); err != nil {
		t.Fatal(err)
	}
	return cfg
}

func TestClient_RefreshNotAttemptedWhenConfigIsNotWritable(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("needs POSIX permissions enforced")
	}
	refreshed := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/auth/refresh" {
			refreshed++
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"at-new","refresh_token":"rt-new","expires_in":3600}`))
	}))
	defer srv.Close()
	dir := t.TempDir()
	expiredBearerConfig(t, dir, srv.URL)
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

	var out any
	ce := CheckResponse(New().Get("/api/v1/accounts", &out))
	if ce == nil || ce.Code != errs.CodeConfigSaveFailed {
		t.Fatalf("got %v, want config_save_failed", ce)
	}
	if refreshed != 0 {
		t.Fatal("refresh sent although the rotated tokens could not be saved")
	}
}

func TestClient_RefreshSaveFailureReturnsTheRotatedRefreshToken(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("needs POSIX permissions enforced")
	}
	dir := t.TempDir()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The directory turns read-only between the writability check and
		// the save: Sure has already revoked rt-old.
		_ = os.Chmod(dir, 0o500)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"at-new","refresh_token":"rt-new","expires_in":3600}`))
	}))
	defer srv.Close()
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })
	expiredBearerConfig(t, dir, srv.URL)

	var out any
	ce := CheckResponse(New().Get("/api/v1/accounts", &out))
	if ce == nil || ce.Code != errs.CodeConfigSaveFailed {
		t.Fatalf("got %v, want config_save_failed", ce)
	}
	if ce.Details["refresh_token"] != "rt-new" {
		t.Fatalf("details = %v, want the rotated refresh_token so the session can be recovered", ce.Details)
	}
	if config.RefreshToken() != "rt-new" || config.Token() != "at-new" {
		t.Fatal("rotated tokens dropped from memory")
	}
}

func TestClient_AdoptsTokensAnotherProcessRotated(t *testing.T) {
	// Two commands refresh at once; Sure rotates for the first and answers
	// the second 401 because rt-old is already revoked. The loser must use
	// the pair the winner saved instead of reporting auth_required.
	var cfg, srvURL string
	refreshes := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/auth/refresh":
			refreshes++
			winner := fmt.Sprintf("api_url: %s\nauth:\n  mode: bearer\n  token: at-winner\n  refresh_token: rt-winner\n  token_expires_at: \"2099-01-01T00:00:00Z\"\n", srvURL)
			if err := os.WriteFile(cfg, []byte(winner), 0o600); err != nil {
				t.Error(err)
			}
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"Invalid refresh token"}`))
		default:
			if got := r.Header.Get("Authorization"); got != "Bearer at-winner" {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
				return
			}
			_, _ = w.Write([]byte(`{"ok":true}`))
		}
	}))
	defer srv.Close()
	srvURL = srv.URL
	cfg = expiredBearerConfig(t, t.TempDir(), srv.URL)

	var out map[string]any
	if ce := CheckResponse(New().Get("/api/v1/usage", &out)); ce != nil {
		t.Fatalf("got %v, want the request to succeed with the winner's token", ce)
	}
	if refreshes != 1 || out["ok"] != true {
		t.Fatalf("refreshes=%d out=%v", refreshes, out)
	}
}

// Sure's API never redirects. Following one would hand the API key, bearer
// token or login password to whatever host the Location names.
func TestClient_RefusesRedirects(t *testing.T) {
	var leaked atomic.Int32
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		leaked.Add(1)
	}))
	t.Cleanup(other.Close)
	otherURL := strings.Replace(other.URL, "127.0.0.1", "localhost", 1)

	for name, tc := range map[string]struct {
		status int
		call   func(c *Client) error
	}{
		"GET": {http.StatusFound, func(c *Client) error {
			var out any
			_, err := c.Get("/api/v1/accounts", &out)
			return err
		}},
		"login POST": {http.StatusTemporaryRedirect, func(c *Client) error {
			_, err := c.Login(LoginRequest{Email: "a@b.c", Password: "pw"})
			return err
		}},
		"download": {http.StatusFound, func(c *Client) error {
			_, err := c.GetToFile(context.Background(), "/dl", filepath.Join(t.TempDir(), "e.zip"), 0)
			return err
		}},
	} {
		t.Run(name, func(t *testing.T) {
			leaked.Store(0)
			var hits atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hits.Add(1)
				http.Redirect(w, r, otherURL+"/sso", tc.status)
			}))
			t.Cleanup(srv.Close)

			ce := assertCode(t, tc.call(testClient(t, srv.URL)), errs.CodeRedirectRefused)
			if leaked.Load() != 0 {
				t.Fatal("the redirect target received the request")
			}
			if hits.Load() != 1 {
				t.Fatalf("requests = %d, want 1 (a refused redirect is not retried)", hits.Load())
			}
			if ce.Details["status"] != tc.status || ce.Details["location"] != otherURL+"/sso" {
				t.Fatalf("details = %v", ce.Details)
			}
		})
	}
}
