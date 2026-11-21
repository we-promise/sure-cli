package root

import (
	"bytes"
	"encoding/json"
	stderrors "errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/we-promise/sure-cli/internal/api"
	errs "github.com/we-promise/sure-cli/internal/errors"
)

func TestLoginCommandHasOTPFlag(t *testing.T) {
	if newLoginCmd().Flags().Lookup("otp") == nil {
		t.Fatal("login must expose --otp (sent as otp_code)")
	}
}

func TestLoginError_MFARequiredWithoutOTPAsksForIt(t *testing.T) {
	sure := errs.ClassifyHTTPError(http.StatusUnauthorized,
		[]byte(`{"error":"Two-factor authentication required","mfa_required":true}`), nil)

	ce := loginError(sure, "")
	if ce.Code != errs.CodeMFARequired {
		t.Fatalf("code = %q", ce.Code)
	}
	if !strings.Contains(ce.Message, "--otp") {
		t.Fatalf("message should tell the user to pass --otp, got %q", ce.Message)
	}
	if ce.Details["mfa_required"] != true {
		t.Fatalf("details lost: %v", ce.Details)
	}
}

func TestLoginError_MFARequiredWithOTPReportsRejectedCode(t *testing.T) {
	sure := errs.ClassifyHTTPError(http.StatusUnauthorized,
		[]byte(`{"error":"Two-factor authentication required","mfa_required":true}`), nil)

	ce := loginError(sure, "000000")
	if ce.Code != errs.CodeMFARequired || !strings.Contains(ce.Message, "rejected") {
		t.Fatalf("got %s %q", ce.Code, ce.Message)
	}
}

func TestLoginError_OtherFailuresKeepSureMessage(t *testing.T) {
	sure := errs.ClassifyHTTPError(http.StatusForbidden,
		[]byte(`{"error":"Local login is disabled. Please use SSO."}`), nil)

	ce := loginError(sure, "")
	if ce.Code != "login_failed" || ce.Message != "Local login is disabled. Please use SSO." {
		t.Fatalf("got %s %q", ce.Code, ce.Message)
	}
	if ce.Details["status"] != http.StatusForbidden {
		t.Fatalf("details = %v", ce.Details)
	}

}

// credentialFailures are the answers with which Sure turns down a login or
// refresh: bad credentials, a disabled login, missing or rejected device info.
var credentialFailures = []int{http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden, http.StatusUnprocessableEntity}

// otherFailures keep their classified code: Sure was not reached, its answer
// was lost or unreadable, or it asked the CLI to wait or look elsewhere.
func otherFailures() map[string]error {
	return map[string]error{
		errs.CodeRedirectRefused: errs.New(errs.CodeRedirectRefused, "redirect").WithDetails(map[string]any{"status": http.StatusMovedPermanently}),
		errs.CodeOutcomeUnknown:  errs.New(errs.CodeOutcomeUnknown, "lost"),
		errs.CodeTimeout:         errs.New(errs.CodeTimeout, "slow"),
		errs.CodeNetwork:         errs.New(errs.CodeNetwork, "refused"),
		errs.CodeRateLimit:       errs.ClassifyHTTPError(http.StatusTooManyRequests, nil, http.Header{"Retry-After": {"60"}}),
		errs.CodeServerError:     errs.ClassifyHTTPError(http.StatusBadGateway, nil, nil),
		errs.CodeNotFound:        errs.ClassifyHTTPError(http.StatusNotFound, nil, nil),
		errs.CodeInvalidResponse: errs.InvalidResponse(http.StatusOK, []byte("<html>"), nil, stderrors.New("html")),
		errs.CodeValidation:      errs.New(errs.CodeValidation, "api_url"),
		errs.CodeUnknown:         stderrors.New("boom"),
	}
}

func TestCredentialErrors_KeepTheCodeOfAFailureToReachSure(t *testing.T) {
	for code, err := range otherFailures() {
		if got := loginError(err, ""); got.Code != code {
			t.Errorf("login: %s became %s", code, got.Code)
		}
		if got := refreshError(err); got.Code != code {
			t.Errorf("refresh: %s became %s", code, got.Code)
		}
	}
	for _, status := range credentialFailures {
		sure := errs.ClassifyHTTPError(status, []byte(`{"error":"Invalid email or password"}`), nil)
		if got := loginError(sure, ""); got.Code != errs.CodeLoginFailed || got.Message != "Invalid email or password" || got.Details["status"] != status {
			t.Errorf("login %d: got %s %q %v", status, got.Code, got.Message, got.Details)
		}
		if got := refreshError(sure); got.Code != errs.CodeRefreshFailed || got.Details["status"] != status {
			t.Errorf("refresh %d: got %s %v", status, got.Code, got.Details)
		}
	}
}

func TestLogin_RedirectIsRedirectRefused(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://example.invalid"+r.URL.Path, http.StatusMovedPermanently)
	}))
	t.Cleanup(srv.Close)
	_, err := apiClientFor(t, srv.URL).Login(api.LoginRequest{Email: "a", Password: "b"})
	if got := loginError(err, ""); got.Code != errs.CodeRedirectRefused {
		t.Fatalf("code = %s, want redirect_refused", got.Code)
	}
}

// Piped into login, an email and a password on two lines both reach Sure, and
// the prompt goes to stderr: stdout holds only the envelope.
func TestLogin_PipedCredentialsPromptOnStderr(t *testing.T) {
	requests := make(chan api.LoginRequest, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req api.LoginRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		requests <- req
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"at","refresh_token":"rt","token_type":"Bearer","expires_in":3600,"created_at":1}`))
	}))
	t.Cleanup(srv.Close)
	cfg := writeConfig(t, "api_url: "+srv.URL+"\n")

	child := cliCommand("--config", cfg, "login")
	child.Stdin = strings.NewReader("a@b.c\nsecret\n")
	var stdout, stderr bytes.Buffer
	child.Stdout, child.Stderr = &stdout, &stderr
	if err := child.Run(); err != nil {
		t.Fatalf("login: %v\nstdout %s\nstderr %s", err, stdout.String(), stderr.String())
	}
	sent := <-requests
	if sent.Email != "a@b.c" || sent.Password != "secret" {
		t.Fatalf("sent email %q password %q, want a@b.c and secret", sent.Email, sent.Password)
	}
	var env map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil || env["data"] == nil {
		t.Fatalf("stdout = %q (%v), want only the envelope", stdout.String(), err)
	}
	if !strings.Contains(stderr.String(), "Email: ") {
		t.Fatalf("stderr = %q, want the email prompt", stderr.String())
	}
}
