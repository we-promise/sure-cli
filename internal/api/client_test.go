package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/we-promise/sure-cli/internal/config"
	"github.com/spf13/viper"
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
