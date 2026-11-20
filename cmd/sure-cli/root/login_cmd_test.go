package root

import (
	stderrors "errors"
	"net/http"
	"strings"
	"testing"

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

	plain := loginError(stderrors.New("boom"), "")
	if plain.Code != "login_failed" || plain.Message != "boom" {
		t.Fatalf("got %s %q", plain.Code, plain.Message)
	}
}
