package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/spf13/viper"

	"github.com/we-promise/sure-cli/internal/config"
	errs "github.com/we-promise/sure-cli/internal/errors"
	"github.com/we-promise/sure-cli/internal/models"
)

func TestLogin_SendsOTPCode(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&got)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"tok","refresh_token":"ref","expires_in":60}`))
	}))
	t.Cleanup(srv.Close)

	c := testClient(t, srv.URL)
	res, err := c.Login(LoginRequest{Email: "a@b.c", Password: "pw", OTPCode: "123456", Device: models.DeviceInfo{DeviceID: "d"}})
	if err != nil {
		t.Fatal(err)
	}
	if res.AccessToken != "tok" {
		t.Fatalf("access_token = %q", res.AccessToken)
	}
	if got["otp_code"] != "123456" {
		t.Fatalf("otp_code = %v", got["otp_code"])
	}
}

func TestLogin_ReturnsClassifiedErrors(t *testing.T) {
	cases := []struct {
		status  int
		body    string
		code    string
		message string
	}{
		{http.StatusUnauthorized, `{"error":"Two-factor authentication required","mfa_required":true}`, errs.CodeMFARequired, "Two-factor authentication required"},
		{http.StatusForbidden, `{"error":"Local login is disabled. Please use SSO."}`, errs.CodeForbidden, "Local login is disabled. Please use SSO."},
		{http.StatusUnauthorized, `{"error":"Invalid email or password"}`, errs.CodeAuthRequired, "Invalid email or password"},
	}
	for _, tc := range cases {
		srv, _ := countingServer(t, tc.status, nil, tc.body)
		c := testClient(t, srv.URL)
		_, err := c.Login(LoginRequest{Email: "a@b.c", Password: "pw"})
		var ce *errs.CLIError
		if !errors.As(err, &ce) {
			t.Fatalf("%s: want *CLIError, got %T %v", tc.body, err, err)
		}
		if ce.Code != tc.code || ce.Message != tc.message {
			t.Fatalf("%s: got %s %q, want %s %q", tc.body, ce.Code, ce.Message, tc.code, tc.message)
		}
	}
}

func TestClient_AutoRefreshFailureKeepsSavedTokens(t *testing.T) {
	srv, hits := countingServer(t, http.StatusUnauthorized, nil, `{"error":"Invalid refresh token"}`)
	viper.Reset()
	_ = config.Init(t.TempDir() + "/config.yaml")
	viper.Set("api_url", srv.URL)
	viper.Set("auth.mode", "bearer")
	viper.Set("auth.token", "tok_old")
	viper.Set("auth.refresh_token", "ref_old")
	viper.Set("auth.token_expires_at", time.Now().Add(-time.Minute).UTC().Format(time.RFC3339))

	var out any
	_, err := New().Get("/api/v1/usage", &out)
	var ce *errs.CLIError
	if !errors.As(err, &ce) || ce.Code != errs.CodeAuthRequired {
		t.Fatalf("got %v, want auth_required from the failed refresh", err)
	}
	if config.Token() != "tok_old" || config.RefreshToken() != "ref_old" {
		t.Fatalf("tokens overwritten: %q %q", config.Token(), config.RefreshToken())
	}
	if hits[http.MethodGet] != 0 {
		t.Fatalf("request sent despite failed refresh")
	}
}

func TestRefresh_ReturnsClassifiedErrors(t *testing.T) {
	srv, _ := countingServer(t, http.StatusUnauthorized, nil, `{"error":"Invalid refresh token"}`)
	c := testClient(t, srv.URL)
	_, err := c.Refresh(RefreshRequest{RefreshToken: "r"})
	var ce *errs.CLIError
	if !errors.As(err, &ce) || ce.Code != errs.CodeAuthRequired || ce.Message != "Invalid refresh token" {
		t.Fatalf("got %v", err)
	}
}
