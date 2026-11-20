package errors

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestClassifyHTTPError_MapsStatusToCode(t *testing.T) {
	// Locks in the stable CLI code taxonomy agents branch on.
	cases := []struct {
		status int
		body   string
		want   string
	}{
		{http.StatusBadRequest, `{"error":"bad_request","message":"Required parameters are missing or invalid"}`, CodeBadRequest},
		{http.StatusUnauthorized, `{"error":"unauthorized","message":"Access token is invalid or expired"}`, CodeAuthRequired},
		{http.StatusUnauthorized, `{"error":"Two-factor authentication required","mfa_required":true}`, CodeMFARequired},
		{http.StatusForbidden, `{"error":"insufficient_scope","message":"This action requires the 'write' scope"}`, CodeAuthInvalid},
		{http.StatusForbidden, `{"error":"forbidden","message":"Guests cannot modify family configuration"}`, CodeForbidden},
		{http.StatusForbidden, `{"error":"feature_disabled","message":"AI features are not enabled for this user"}`, CodeForbidden},
		{http.StatusForbidden, `{"error":"Local login is disabled. Please use SSO."}`, CodeForbidden},
		{http.StatusNotFound, `{"error":"record_not_found","message":"The requested resource was not found"}`, CodeNotFound},
		{http.StatusConflict, `{"error":"export_not_ready","message":"Export is not ready for download"}`, CodeConflict},
		{http.StatusUnprocessableEntity, `{"error":"validation_failed","message":"Name can't be blank","errors":["Name can't be blank"]}`, CodeValidation},
		{http.StatusTooManyRequests, `{"error":"rate_limit_exceeded"}`, CodeRateLimit},
		{http.StatusInternalServerError, `{"error":"publish_failed","import_id":"imp_1"}`, CodeServerError},
		{http.StatusBadGateway, `<html>bad gateway</html>`, CodeServerError},
		{http.StatusServiceUnavailable, ``, CodeServerError},
		{http.StatusTeapot, ``, CodeUnknown},
	}
	for _, c := range cases {
		ce := ClassifyHTTPError(c.status, []byte(c.body), nil)
		if ce.Code != c.want {
			t.Errorf("status %d body %s: code = %q, want %q", c.status, c.body, ce.Code, c.want)
		}
	}
}

func TestClassifyHTTPError_SurfacesSureErrorCodeAndMessage(t *testing.T) {
	ce := ClassifyHTTPError(http.StatusForbidden,
		[]byte(`{"error":"self_hosted_only","message":"Only available on self-hosted instances"}`), nil)
	if ce.Message != "Only available on self-hosted instances" {
		t.Fatalf("message = %q, want Sure's message", ce.Message)
	}
	if ce.Details["api_error"] != "self_hosted_only" {
		t.Fatalf("details.api_error = %v", ce.Details["api_error"])
	}
	if ce.Details["status"] != http.StatusForbidden {
		t.Fatalf("details.status = %v", ce.Details["status"])
	}
	if !strings.Contains(ce.Details["body"].(string), "self_hosted_only") {
		t.Fatalf("details.body = %v", ce.Details["body"])
	}
}

func TestClassifyHTTPError_HumanErrorStringBecomesMessage(t *testing.T) {
	// AuthController renders {error: "<sentence>"} without a code.
	ce := ClassifyHTTPError(http.StatusForbidden, []byte(`{"error":"Local login is disabled. Please use SSO."}`), nil)
	if ce.Message != "Local login is disabled. Please use SSO." {
		t.Fatalf("message = %q", ce.Message)
	}
	if _, ok := ce.Details["api_error"]; ok {
		t.Fatalf("a sentence is not an error code; details.api_error = %v", ce.Details["api_error"])
	}
}

func TestClassifyHTTPError_KeepsSureStructuredFields(t *testing.T) {
	ce := ClassifyHTTPError(http.StatusUnprocessableEntity,
		[]byte(`{"error":"validation_failed","message":"bad","errors":["Name can't be blank"],"details":{"field":"name"}}`), nil)
	if errs, _ := ce.Details["errors"].([]any); len(errs) != 1 || errs[0] != "Name can't be blank" {
		t.Fatalf("details.errors = %#v", ce.Details["errors"])
	}
	if d, _ := ce.Details["api_details"].(map[string]any); d["field"] != "name" {
		t.Fatalf("details.api_details = %#v", ce.Details["api_details"])
	}

	mfa := ClassifyHTTPError(http.StatusUnauthorized, []byte(`{"error":"Two-factor authentication required","mfa_required":true}`), nil)
	if mfa.Details["mfa_required"] != true {
		t.Fatalf("details.mfa_required = %v", mfa.Details["mfa_required"])
	}
}

func TestClassifyHTTPError_FallsBackToDefaultMessageWithoutJSON(t *testing.T) {
	ce := ClassifyHTTPError(http.StatusBadGateway, []byte("<html>bad gateway</html>"), nil)
	if ce.Message != "Server error (HTTP 502)" {
		t.Fatalf("message = %q", ce.Message)
	}
	if _, ok := ce.Details["api_error"]; ok {
		t.Fatal("no api_error expected for a non-JSON body")
	}
	if ce.Details["body"] != "<html>bad gateway</html>" {
		t.Fatalf("details.body = %v", ce.Details["body"])
	}
}

func TestClassifyHTTPError_RateLimitHeaders(t *testing.T) {
	h := http.Header{}
	h.Set("Retry-After", "42")
	h.Set("X-RateLimit-Limit", "100")
	h.Set("X-RateLimit-Remaining", "0")
	h.Set("X-RateLimit-Reset", "42")
	ce := ClassifyHTTPError(http.StatusTooManyRequests,
		[]byte(`{"error":"rate_limit_exceeded","message":"Rate limit exceeded. Try again in 42 seconds."}`), h)

	if ce.Details["retry_after"] != "42" {
		t.Fatalf("details.retry_after = %v", ce.Details["retry_after"])
	}
	rl, _ := ce.Details["rate_limit"].(map[string]string)
	if rl["limit"] != "100" || rl["remaining"] != "0" || rl["reset"] != "42" {
		t.Fatalf("details.rate_limit = %#v", ce.Details["rate_limit"])
	}
	if ce.Message != "Rate limit exceeded. Try again in 42 seconds." {
		t.Fatalf("message = %q", ce.Message)
	}

	plain := ClassifyHTTPError(http.StatusTooManyRequests, nil, nil)
	if _, ok := plain.Details["rate_limit"]; ok {
		t.Fatal("rate_limit should be omitted when no headers are present")
	}
}

func TestClassifyHTTPError_CapsBody(t *testing.T) {
	body := strings.Repeat("x", MaxBodyBytes+500)
	ce := ClassifyHTTPError(http.StatusUnprocessableEntity, []byte(body), nil)
	got, _ := ce.Details["body"].(string)
	if !strings.HasSuffix(got, "...") || len(got) != MaxBodyBytes+len("...") {
		t.Fatalf("body should be capped at %d bytes plus marker, got %d", MaxBodyBytes, len(got))
	}
	if _, err := json.Marshal(ce.Details); err != nil {
		t.Fatalf("details not JSON-encodable: %v", err)
	}
}

func TestFrom(t *testing.T) {
	ce := New(CodeConflict, "x")
	if got := From(fmt.Errorf("wrap: %w", ce)); got != ce {
		t.Fatalf("From should unwrap to the CLIError, got %+v", got)
	}
	plain := stderrors.New("boom")
	got := From(plain)
	if got.Code != CodeUnknown || got.Message != "boom" || got.Cause != plain {
		t.Fatalf("From(plain) = %+v", got)
	}
}

func TestClassifyHTTPError_OmitsEmptyBody(t *testing.T) {
	ce := ClassifyHTTPError(http.StatusNotFound, []byte("  \n"), nil)
	if _, ok := ce.Details["body"]; ok {
		t.Fatalf("empty body should be omitted, got %v", ce.Details["body"])
	}
	if ce.Details["status"] != http.StatusNotFound {
		t.Fatalf("details.status = %v", ce.Details["status"])
	}
}

func TestClassifyHTTPError_CapKeepsUTF8Valid(t *testing.T) {
	// The cap falls inside the two-byte "é": cutting there would emit an
	// invalid rune into the JSON envelope.
	body := strings.Repeat("a", MaxBodyBytes-1) + "é rest"
	got, _ := ClassifyHTTPError(http.StatusBadGateway, []byte(body), nil).Details["body"].(string)
	if !utf8.ValidString(got) {
		t.Fatalf("capped body is not valid UTF-8: %q", got[len(got)-8:])
	}
	if want := strings.Repeat("a", MaxBodyBytes-1) + "..."; got != want {
		t.Fatalf("capped body should end before the split rune, got tail %q", got[len(got)-8:])
	}
}

func TestClassifyNetworkError_Canceled(t *testing.T) {
	ce := ClassifyNetworkError(fmt.Errorf("get /x: %w", context.Canceled))
	if ce.Code != CodeCanceled {
		t.Fatalf("code = %q, want %q", ce.Code, CodeCanceled)
	}
	if ce := ClassifyNetworkError(fmt.Errorf("get /x: %w", context.DeadlineExceeded)); ce.Code != CodeTimeout {
		t.Fatalf("deadline code = %q, want %q", ce.Code, CodeTimeout)
	}
}

func TestInvalidResponse_CarriesStatusContentTypeAndCappedBody(t *testing.T) {
	h := http.Header{}
	h.Set("Content-Type", "text/html")
	cause := stderrors.New("invalid character '<'")
	ce := InvalidResponse(http.StatusCreated, []byte("<html>"+strings.Repeat("x", MaxBodyBytes)+"</html>"), h, cause)
	if ce.Code != CodeInvalidResponse || ce.Cause != cause {
		t.Fatalf("got %+v", ce)
	}
	if ce.Details["status"] != http.StatusCreated || ce.Details["content_type"] != "text/html" {
		t.Fatalf("details = %v", ce.Details)
	}
	if b, _ := ce.Details["body"].(string); len(b) != MaxBodyBytes+len("...") {
		t.Fatalf("body not capped: %d bytes", len(b))
	}
}
