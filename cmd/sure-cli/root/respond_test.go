package root

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/viper"

	"github.com/we-promise/sure-cli/internal/api"
	"github.com/we-promise/sure-cli/internal/config"
	errs "github.com/we-promise/sure-cli/internal/errors"
)

// apiClientFor returns a fresh api.Client targeting the given test server URL
// by wiring viper directly (mirrors internal/api/client_test.go).
func apiClientFor(t *testing.T, url string) *api.Client {
	t.Helper()
	viper.Reset()
	viper.Set("api_url", url)
	viper.Set("auth.mode", "api_key")
	viper.Set("auth.api_key", "test-key")
	_ = config.Init("/tmp/sure-cli-test-does-not-exist.yaml")
	return api.New()
}

// These tests exercise the building blocks of respond() — the HTTP-error
// classifier wiring and mergeErrorDetails — without going through
// output.Fail (which os.Exits and cannot be intercepted without refactoring
// out of scope for this PR). End-to-end coverage of the success path is
// already provided by TestDispatchWrite_DryRun_* in dispatch_write_test.go;
// end-to-end coverage of the error path is provided by the upstream Sure
// API test suite that hits these classifications in CI smoke runs.

func TestClassifyHTTPError_MapsStatusToCode(t *testing.T) {
	// Locks in the contract respond() relies on: every HTTP status that the
	// CLI cares about must produce a stable, agent-parseable code.
	cases := []struct {
		status int
		want   string
	}{
		{http.StatusUnauthorized, "auth_required"},
		{http.StatusForbidden, "auth_invalid"},
		{http.StatusNotFound, "not_found"},
		{http.StatusUnprocessableEntity, "validation_failed"},
		{http.StatusTooManyRequests, "rate_limited"},
		{http.StatusInternalServerError, "server_error"},
		{http.StatusBadGateway, "server_error"},
		{http.StatusServiceUnavailable, "server_error"},
	}
	for _, c := range cases {
		ce := errs.ClassifyHTTPError(c.status, "body")
		if ce.Code != c.want {
			t.Fatalf("status %d: code = %q, want %q", c.status, ce.Code, c.want)
		}
	}
}

func TestMergeErrorDetails_AlwaysIncludesStatusAndCappedBody(t *testing.T) {
	// Drive a real resty response through the helper so the test catches
	// any regression in either resty's r.String() contract or the cap logic.
	// Use 422 to skip resty's 5xx retry loop (configured in api.New).
	body := strings.Repeat("x", maxRespondBodyBytes+500)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)

	c := apiClientFor(t, srv.URL)
	var discard any
	r, _ := c.Get("/probe", &discard)
	if r == nil {
		t.Fatal("expected non-nil resty response from httptest server")
	}

	details := mergeErrorDetails(nil, r)
	if details["status"] != r.StatusCode() {
		t.Fatalf("status missing/wrong: %v", details["status"])
	}
	gotBody, _ := details["body"].(string)
	if !strings.HasSuffix(gotBody, "...") {
		t.Fatalf("expected truncation marker on long body, got %q", gotBody)
	}
	// The cap is in bytes; allow a few-byte slack for the "..." suffix.
	if len(gotBody) > maxRespondBodyBytes+10 {
		t.Fatalf("body should be capped at ~%d bytes, got %d", maxRespondBodyBytes, len(gotBody))
	}
	// Sanity: details must JSON-encode cleanly (the envelope path requires it).
	if _, err := json.Marshal(details); err != nil {
		t.Fatalf("details not JSON-encodable: %v", err)
	}
}

func TestMergeErrorDetails_PreservesClassifierDetails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"errors":["color is invalid"]}`))
	}))
	t.Cleanup(srv.Close)

	c := apiClientFor(t, srv.URL)
	var discard any
	r, _ := c.Get("/probe", &discard)
	ce := errs.ClassifyHTTPError(r.StatusCode(), r.String())

	details := mergeErrorDetails(ce.Details, r)
	if details["status"] != http.StatusUnprocessableEntity {
		t.Fatalf("status = %v", details["status"])
	}
	if !strings.Contains(details["body"].(string), "color is invalid") {
		t.Fatalf("body should contain the 422 payload, got %v", details["body"])
	}
	// The classifier attaches its own "body" key for 422; verify our merge
	// kept the raw response body in details["body"] (classifier's "body"
	// override has the truncated copy too).
}
