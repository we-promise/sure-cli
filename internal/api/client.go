package api

import (
	"fmt"
	"strings"
	"time"

	"github.com/we-promise/sure-cli/internal/config"
	"github.com/go-resty/resty/v2"
)

type Client struct {
	http       *resty.Client
	refreshing bool
}

func New() *Client {
	c := resty.New().
		SetBaseURL(strings.TrimRight(config.APIURL(), "/")).
		SetTimeout(30*time.Second).
		SetHeader("Accept", "application/json").
		SetRetryCount(2).
		SetRetryWaitTime(1 * time.Second).
		SetRetryMaxWaitTime(5 * time.Second).
		AddRetryCondition(func(r *resty.Response, err error) bool {
			// Retry on network errors or 5xx
			if err != nil {
				return true
			}
			return r.StatusCode() >= 500
		})

	// Auth
	switch config.AuthMode() {
	case "api_key":
		if k := config.APIKey(); k != "" {
			c.SetHeader("X-Api-Key", k)
		}
	default:
		if t := config.Token(); t != "" {
			c.SetHeader("Authorization", fmt.Sprintf("Bearer %s", t))
		}
	}

	return &Client{http: c}
}

func (c *Client) ensureFreshToken() error {
	// Only for bearer auth.
	if config.AuthMode() == "api_key" {
		return nil
	}
	if c.refreshing {
		return nil
	}

	expiresAt, ok := config.TokenExpiresAt()
	if !ok {
		return nil
	}
	// Refresh slightly before expiry.
	if time.Until(expiresAt) > 60*time.Second {
		return nil
	}

	rt := config.RefreshToken()
	if rt == "" {
		return nil
	}

	c.refreshing = true
	defer func() { c.refreshing = false }()

	var res TokenResponse
	_, err := c.http.R().
		SetBody(RefreshRequest{RefreshToken: rt, Device: config.Device()}).
		SetResult(&res).
		Post("/api/v1/auth/refresh")
	if err != nil {
		return err
	}

	// Persist + update header
	config.SetToken(res.AccessToken)
	config.SetRefreshToken(res.RefreshToken)
	config.SetTokenExpiresAt(time.Now().Add(time.Duration(res.ExpiresIn) * time.Second))
	if err := config.Save(); err != nil {
		return err
	}
	if res.AccessToken != "" {
		c.http.SetHeader("Authorization", fmt.Sprintf("Bearer %s", res.AccessToken))
	}
	return nil
}

func (c *Client) Get(path string, out any) (*resty.Response, error) {
	if err := c.ensureFreshToken(); err != nil {
		return nil, err
	}
	return c.http.R().SetResult(out).Get(path)
}

func (c *Client) Post(path string, body any, out any) (*resty.Response, error) {
	if err := c.ensureFreshToken(); err != nil {
		return nil, err
	}
	req := c.http.R().SetBody(body)
	if out != nil {
		req = req.SetResult(out)
	}
	return req.Post(path)
}

func (c *Client) Put(path string, body any, out any) (*resty.Response, error) {
	if err := c.ensureFreshToken(); err != nil {
		return nil, err
	}
	req := c.http.R().SetBody(body)
	if out != nil {
		req = req.SetResult(out)
	}
	return req.Put(path)
}

func (c *Client) Delete(path string, out any) (*resty.Response, error) {
	if err := c.ensureFreshToken(); err != nil {
		return nil, err
	}
	req := c.http.R()
	if out != nil {
		req = req.SetResult(out)
	}
	return req.Delete(path)
}
