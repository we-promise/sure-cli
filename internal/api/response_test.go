package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	errs "github.com/we-promise/sure-cli/internal/errors"
)

func TestCheckResponse_Success(t *testing.T) {
	srv, _ := countingServer(t, http.StatusOK, nil, `{"ok":true}`)
	c := testClient(t, srv.URL)
	var out any
	if ce := CheckResponse(c.Get("/x", &out)); ce != nil {
		t.Fatalf("expected nil, got %v", ce)
	}
}

func TestCheckResponse_ClassifiesHTTPErrorWithSureBodyAndHeaders(t *testing.T) {
	srv, _ := countingServer(t, http.StatusTooManyRequests,
		map[string]string{"Retry-After": "3600", "X-RateLimit-Limit": "100"},
		`{"error":"rate_limit_exceeded","message":"Rate limit exceeded. Try again in 3600 seconds."}`)
	c := testClient(t, srv.URL)
	var out any
	ce := CheckResponse(c.Get("/x", &out))
	if ce == nil {
		t.Fatal("expected error")
	}
	if ce.Code != errs.CodeRateLimit || ce.Details["api_error"] != "rate_limit_exceeded" {
		t.Fatalf("got %+v", ce)
	}
	if ce.Details["retry_after"] != "3600" {
		t.Fatalf("headers not forwarded to classifier: %+v", ce.Details)
	}
}

func TestCheckResponse_NetworkError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := srv.URL
	srv.Close()
	c := testClient(t, url)
	var out any
	ce := CheckResponse(c.Post("/x", map[string]any{}, &out))
	if ce == nil || ce.Code != errs.CodeNetwork {
		t.Fatalf("got %+v, want network_error", ce)
	}
}

func TestCheckResponse_PassesThroughCLIError(t *testing.T) {
	want := errs.New(errs.CodeConflict, "not ready")
	got := CheckResponse(nil, fmt.Errorf("wrapped: %w", want))
	if got != want {
		t.Fatalf("got %+v, want the wrapped CLIError unchanged", got)
	}
}
