package api

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/we-promise/sure-cli/internal/config"
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
	r, err := c.http.R().
		SetBody(RefreshRequest{RefreshToken: rt, Device: config.Device()}).
		SetResult(&res).
		Post("/api/v1/auth/refresh")
	if err != nil {
		return err
	}
	// Treat any 4xx/5xx as a refresh failure. Without this guard, a 401 body
	// would decode into an empty TokenResponse and we'd persist empty tokens,
	// silently logging the user out and corrupting saved state.
	if r != nil && r.StatusCode() >= 400 {
		return fmt.Errorf("token refresh failed: HTTP %d", r.StatusCode())
	}
	if res.AccessToken == "" {
		return fmt.Errorf("token refresh returned empty access token")
	}

	// Persist + update header. Guard each rotation field so that a partial
	// response (e.g. server omits refresh_token if rotation is off) doesn't
	// wipe the saved value.
	config.SetToken(res.AccessToken)
	if res.RefreshToken != "" {
		config.SetRefreshToken(res.RefreshToken)
	}
	if res.ExpiresIn > 0 {
		config.SetTokenExpiresAt(time.Now().Add(time.Duration(res.ExpiresIn) * time.Second))
	}
	if err := config.Save(); err != nil {
		return err
	}
	c.http.SetHeader("Authorization", fmt.Sprintf("Bearer %s", res.AccessToken))
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

// PostMultipart sends a multipart/form-data POST. fileContentType, when
// non-empty, sets an explicit Content-Type on the file part — required for
// Sure's import endpoints, whose Import::ALLOWED_CSV_MIME_TYPES and
// SureImport::ALLOWED_NDJSON_CONTENT_TYPES allow-lists do exact-match
// `include?` checks that reject the charset suffix resty's default
// auto-detection emits (e.g. "text/plain; charset=utf-8"). Pass "" to keep
// the default detection behavior.
func (c *Client) PostMultipart(path string, fields map[string]string, fileField, filePath, fileContentType string, out any) (*resty.Response, error) {
	if err := c.ensureFreshToken(); err != nil {
		return nil, err
	}
	if (fileField == "") != (filePath == "") {
		return nil, fmt.Errorf("fileField and filePath must be provided together")
	}
	req := c.http.R()
	if len(fields) > 0 {
		req = req.SetFormData(fields)
	}
	if fileField != "" && filePath != "" {
		if fileContentType != "" {
			// SetMultipartField requires an io.Reader; open the file and let
			// resty close it via the request lifecycle.
			f, err := os.Open(filePath)
			if err != nil {
				return nil, fmt.Errorf("open file: %w", err)
			}
			defer f.Close()
			req = req.SetMultipartField(fileField, filepath.Base(filePath), fileContentType, f)
		} else {
			req = req.SetFile(fileField, filePath)
		}
	}
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

func (c *Client) Patch(path string, body any, out any) (*resty.Response, error) {
	if err := c.ensureFreshToken(); err != nil {
		return nil, err
	}
	req := c.http.R().SetBody(body)
	if out != nil {
		req = req.SetResult(out)
	}
	return req.Patch(path)
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

func (c *Client) GetToFile(path, outputPath string) (*resty.Response, error) {
	if err := c.ensureFreshToken(); err != nil {
		return nil, err
	}
	return c.http.R().SetOutput(outputPath).Get(path)
}
