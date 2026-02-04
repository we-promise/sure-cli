package api

import (
	"fmt"
	"strings"

	"github.com/dgilperez/sure-cli/internal/models"
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

func (c *Client) Login(req LoginRequest) (LoginResponse, error) {
	var res LoginResponse
	r, err := c.Post("/api/v1/auth/login", req, &res)
	if err != nil {
		return res, err
	}
	if r.StatusCode() >= 400 {
		return res, fmt.Errorf("login failed: status %d: %s", r.StatusCode(), strings.TrimSpace(r.String()))
	}
	return res, nil
}

func (c *Client) Refresh(req RefreshRequest) (TokenResponse, error) {
	var res TokenResponse
	r, err := c.Post("/api/v1/auth/refresh", req, &res)
	if err != nil {
		return res, err
	}
	if r.StatusCode() >= 400 {
		return res, fmt.Errorf("refresh failed: status %d: %s", r.StatusCode(), strings.TrimSpace(r.String()))
	}
	return res, nil
}
