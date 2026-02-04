package api

import (
	"fmt"
	"strings"
	"time"

	"github.com/dgilperez/sure-cli/internal/config"
	"github.com/go-resty/resty/v2"
)

type Client struct {
	http *resty.Client
}

func New() *Client {
	c := resty.New().
		SetBaseURL(strings.TrimRight(config.APIURL(), "/")).
		SetTimeout(30*time.Second).
		SetHeader("Accept", "application/json")

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

func (c *Client) Get(path string, out any) (*resty.Response, error) {
	return c.http.R().SetResult(out).Get(path)
}

func (c *Client) Post(path string, body any, out any) (*resty.Response, error) {
	req := c.http.R().SetBody(body)
	if out != nil {
		req = req.SetResult(out)
	}
	return req.Post(path)
}

func (c *Client) Put(path string, body any, out any) (*resty.Response, error) {
	req := c.http.R().SetBody(body)
	if out != nil {
		req = req.SetResult(out)
	}
	return req.Put(path)
}

func (c *Client) Delete(path string, out any) (*resty.Response, error) {
	req := c.http.R()
	if out != nil {
		req = req.SetResult(out)
	}
	return req.Delete(path)
}
