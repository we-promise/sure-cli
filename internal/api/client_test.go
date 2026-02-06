package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/we-promise/sure-cli/internal/config"
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
	_, err := c.PostMultipart("/api/v1/imports", fields, "file", tmpFile, &out)
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
	_, err := c.PostMultipart("/api/v1/imports", nil, "file", "", nil)
	if err == nil {
		t.Fatal("expected error for mismatched file arguments")
	}

	// filePath set but fileField empty
	_, err = c.PostMultipart("/api/v1/imports", nil, "", "/some/path", nil)
	if err == nil {
		t.Fatal("expected error for mismatched file arguments")
	}
}
