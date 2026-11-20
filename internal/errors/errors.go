package errors

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"regexp"
	"strings"
)

// Error codes for agent-friendly error handling
const (
	CodeAuthRequired  = "auth_required"
	CodeAuthInvalid   = "auth_invalid"
	CodeAuthExpired   = "auth_expired"
	CodeMFARequired   = "mfa_required"
	CodeForbidden     = "forbidden"
	CodeBadRequest    = "bad_request"
	CodeConflict      = "conflict"
	CodeNotFound      = "not_found"
	CodeValidation    = "validation_failed"
	CodeNetwork       = "network_error"
	CodeTimeout       = "timeout"
	CodeRateLimit     = "rate_limited"
	CodeServerError   = "server_error"
	CodeConfigMissing = "config_missing"
	CodeConfigInvalid = "config_invalid"
	CodeExportFailed  = "export_failed"
	// CodeInvalidResponse means Sure answered 2xx with a body the CLI cannot read.
	CodeInvalidResponse = "invalid_response"
	CodeUnknown         = "unknown_error"
)

// CLIError is a structured error with code and details for agent consumption.
type CLIError struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
	Cause   error          `json:"-"`
}

func (e *CLIError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s (%v)", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *CLIError) Unwrap() error {
	return e.Cause
}

// New creates a new CLIError
func New(code, message string) *CLIError {
	return &CLIError{Code: code, Message: message}
}

// Wrap wraps an existing error with a CLIError
func Wrap(code, message string, cause error) *CLIError {
	return &CLIError{Code: code, Message: message, Cause: cause}
}

// From returns the *CLIError in err's chain, or wraps err as unknown_error.
func From(err error) *CLIError {
	var ce *CLIError
	if errors.As(err, &ce) {
		return ce
	}
	return Wrap(CodeUnknown, err.Error(), err)
}

// WithDetails adds details to the error
func (e *CLIError) WithDetails(details map[string]any) *CLIError {
	e.Details = details
	return e
}

// MaxBodyBytes caps the upstream response body kept in error details. Sure
// may echo the original request (including the email or OTP for auth flows)
// in 4xx bodies — keep enough for diagnosis without bulk.
const MaxBodyBytes = 1024

// sureErrorBody is the JSON error shape Sure's API renders:
// {error: "<code>", message: "...", details?: {...}, errors?: [...]}.
// AuthController instead puts a human sentence in `error` and may add
// `mfa_required: true`. ImportsController adds `import_id` when it created
// the import before failing.
type sureErrorBody struct {
	Error       string `json:"error"`
	Message     string `json:"message"`
	Details     any    `json:"details"`
	Errors      any    `json:"errors"`
	MFARequired bool   `json:"mfa_required"`
	ImportID    any    `json:"import_id"`
}

var sureErrorCode = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// ClassifyHTTPError maps an HTTP error response to a CLIError. Code follows
// the stable CLI taxonomy (driven by status); Message is Sure's own message
// when it sends one. Details always carry `status` and the capped `body`,
// plus, when Sure provides them, `api_error` (Sure's machine code, e.g.
// insufficient_scope, export_not_ready), `api_details`, `errors`,
// `mfa_required`, `import_id` (the import Sure created before failing; do
// not repeat the POST), and on 429 `retry_after` / `rate_limit`.
func ClassifyHTTPError(status int, body []byte, header http.Header) *CLIError {
	details := map[string]any{"status": status}
	if b := strings.TrimSpace(string(body)); b != "" {
		details["body"] = truncate(b, MaxBodyBytes)
	}

	var sure sureErrorBody
	_ = json.Unmarshal(body, &sure) // non-JSON bodies keep the zero value
	apiError, message := "", sure.Message
	if sureErrorCode.MatchString(sure.Error) {
		apiError = sure.Error
		details["api_error"] = apiError
	} else if message == "" {
		message = sure.Error
	}
	if sure.Details != nil {
		details["api_details"] = sure.Details
	}
	if sure.Errors != nil {
		details["errors"] = sure.Errors
	}
	if sure.ImportID != nil {
		details["import_id"] = sure.ImportID
	}
	if sure.MFARequired {
		details["mfa_required"] = true
	}

	code, fallback := classifyStatus(status, apiError, sure.MFARequired)
	if status == http.StatusTooManyRequests {
		addRateLimitDetails(details, header)
	}
	if message == "" {
		message = fallback
	}
	return New(code, message).WithDetails(details)
}

// classifyStatus returns the CLI code and default message for a status.
func classifyStatus(status int, apiError string, mfaRequired bool) (code, message string) {
	switch {
	case status == http.StatusBadRequest:
		return CodeBadRequest, "Bad request"
	case status == http.StatusUnauthorized && mfaRequired:
		return CodeMFARequired, "Two-factor authentication required"
	case status == http.StatusUnauthorized:
		return CodeAuthRequired, "Authentication required"
	case status == http.StatusForbidden && apiError == "insufficient_scope":
		return CodeAuthInvalid, "Access denied: insufficient scope"
	case status == http.StatusForbidden:
		return CodeForbidden, "Access denied"
	case status == http.StatusNotFound:
		return CodeNotFound, "Resource not found"
	case status == http.StatusConflict:
		return CodeConflict, "Conflict"
	case status == http.StatusUnprocessableEntity:
		return CodeValidation, "Validation failed"
	case status == http.StatusTooManyRequests:
		return CodeRateLimit, "Rate limit exceeded"
	case status >= 500:
		return CodeServerError, fmt.Sprintf("Server error (HTTP %d)", status)
	default:
		return CodeUnknown, fmt.Sprintf("HTTP %d", status)
	}
}

// addRateLimitDetails copies Sure's Retry-After and X-RateLimit-* headers so
// agents can schedule the next attempt.
func addRateLimitDetails(details map[string]any, header http.Header) {
	if v := header.Get("Retry-After"); v != "" {
		details["retry_after"] = v
	}
	rl := map[string]string{}
	for key, h := range map[string]string{"limit": "X-RateLimit-Limit", "remaining": "X-RateLimit-Remaining", "reset": "X-RateLimit-Reset"} {
		if v := header.Get(h); v != "" {
			rl[key] = v
		}
	}
	if len(rl) > 0 {
		details["rate_limit"] = rl
	}
}

// ClassifyNetworkError classifies network errors
func ClassifyNetworkError(err error) *CLIError {
	if err == nil {
		return nil
	}

	// Check for timeout
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return Wrap(CodeTimeout, "Request timed out", err)
	}

	// Check for connection errors
	if strings.Contains(err.Error(), "connection refused") {
		return Wrap(CodeNetwork, "Connection refused - is the server running?", err)
	}
	if strings.Contains(err.Error(), "no such host") {
		return Wrap(CodeNetwork, "Server not found - check api_url config", err)
	}

	return Wrap(CodeNetwork, "Network error", err)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
