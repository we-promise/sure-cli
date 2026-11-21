package api

import (
	"github.com/we-promise/sure-cli/internal/models"
)

type LoginRequest struct {
	Email    string            `json:"email"`
	Password string            `json:"password"`
	OTPCode  string            `json:"otp_code,omitempty"`
	Device   models.DeviceInfo `json:"device"`
}

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	CreatedAt    int64  `json:"created_at"`
}

type LoginResponse struct {
	TokenResponse
	User map[string]any `json:"user"`
}

type RefreshRequest struct {
	RefreshToken string            `json:"refresh_token"`
	Device       models.DeviceInfo `json:"device"`
}

// Login exchanges credentials for tokens. Failures are *errors.CLIError, so
// callers can branch on Sure's MFA challenge (Code == mfa_required) and show
// Sure's own message (e.g. "Local login is disabled. Please use SSO.").
func (c *Client) Login(req LoginRequest) (LoginResponse, error) {
	var res LoginResponse
	if ce := CheckResponse(c.postAuth("/api/v1/auth/login", req, &res)); ce != nil {
		return res, ce
	}
	return res, nil
}

// Refresh rotates the token pair. Failures are *errors.CLIError.
func (c *Client) Refresh(req RefreshRequest) (TokenResponse, error) {
	var res TokenResponse
	if ce := CheckResponse(c.postAuth("/api/v1/auth/refresh", req, &res)); ce != nil {
		return res, ce
	}
	return res, nil
}
