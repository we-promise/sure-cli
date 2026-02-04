package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dgilperez/sure-cli/internal/config"
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
