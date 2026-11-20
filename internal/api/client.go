package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/we-promise/sure-cli/internal/config"
	errs "github.com/we-promise/sure-cli/internal/errors"
)

// MaxResponseBytes caps a buffered API response; a larger body is an
// invalid_response rather than an unbounded allocation. Downloads stream to
// disk and are not capped.
const MaxResponseBytes = 64 << 20

type Client struct {
	http       *resty.Client
	timeout    time.Duration
	refreshing bool
}

func New() *Client {
	timeout := config.Timeout()
	c := resty.New().
		SetBaseURL(strings.TrimRight(config.APIURL(), "/")).
		SetHeader("Accept", "application/json").
		SetResponseBodyLimit(MaxResponseBytes).
		SetRetryCount(2).
		SetRetryWaitTime(1 * time.Second).
		SetRetryMaxWaitTime(5 * time.Second).
		SetRedirectPolicy(resty.RedirectPolicyFunc(refuseRedirect))
	// No http.Client timeout: it would also cut a download that is still
	// flowing. Calls are bounded by a context deadline (see do) and the wait
	// for response headers by the transport.
	if t, err := c.Transport(); err == nil {
		t.ResponseHeaderTimeout = timeout
	}
	c.AddRetryCondition(func(r *resty.Response, err error) bool {
		return shouldRetry(r, err, c.RetryMaxWaitTime) && fitsDeadline(r, retryWait(r, c.RetryWaitTime, c.RetryMaxWaitTime))
	}).SetRetryAfter(func(c *resty.Client, r *resty.Response) (time.Duration, error) {
		return retryWait(r, c.RetryWaitTime, c.RetryMaxWaitTime), nil
	})

	// Auth
	switch config.AuthMode() {
	case "api_key":
		if k := config.APIKey(); k != "" {
			c.SetHeader("X-Api-Key", k)
		}
	default:
		if t := config.Token(); t != "" {
			c.SetHeader("Authorization", "Bearer "+t)
		}
	}

	return &Client{http: c, timeout: timeout}
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

	// Sure revokes rt as soon as it rotates the pair, so a pair the CLI
	// cannot persist logs the user out: refuse to rotate into nowhere.
	if err := config.CheckWritable(); err != nil {
		return errs.Wrap(errs.CodeConfigSaveFailed, "config file is not writable; cannot refresh the access token", err)
	}

	c.refreshing = true
	defer func() { c.refreshing = false }()

	// Refresh fails on any 4xx/5xx. Without that, a 401 body would decode
	// into an empty TokenResponse and we'd persist empty tokens, silently
	// logging the user out and corrupting saved state.
	res, err := c.Refresh(RefreshRequest{RefreshToken: rt, Device: config.Device()})
	if err != nil {
		if errs.From(err).Code == errs.CodeAuthRequired && c.adoptRotatedTokens(rt) {
			return nil
		}
		return err
	}
	if res.AccessToken == "" {
		return errs.New(errs.CodeInvalidResponse, "token refresh returned empty access token")
	}

	// Guard each rotation field so that a partial response (e.g. server
	// omits refresh_token if rotation is off) doesn't wipe the saved value.
	config.SetToken(res.AccessToken)
	if res.RefreshToken != "" {
		config.SetRefreshToken(res.RefreshToken)
	}
	if res.ExpiresIn > 0 {
		config.SetTokenExpiresAt(time.Now().Add(time.Duration(res.ExpiresIn) * time.Second))
	}
	c.setBearer(res.AccessToken)
	if err := config.Save(); err != nil {
		// rt is already revoked; the new refresh token is the only way back
		// in without a new login.
		return errs.Wrap(errs.CodeConfigSaveFailed, "token refreshed but the config file could not be saved; set auth.refresh_token to details.refresh_token or log in again", err).
			WithDetails(map[string]any{"refresh_token": config.RefreshToken()})
	}
	return nil
}

// rotationWait is how long a refresh that lost a race waits for the winning
// process to save the pair Sure rotated for it.
const rotationWait = time.Second

// adoptRotatedTokens handles a refresh rejected because another sure-cli
// process rotated rt first: it re-reads the config until that process has
// saved the new pair and switches to it.
func (c *Client) adoptRotatedTokens(rt string) bool {
	for deadline := time.Now().Add(rotationWait); ; time.Sleep(rotationWait / 10) {
		if config.Reload() == nil && config.RefreshToken() != rt && config.Token() != "" {
			c.setBearer(config.Token())
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
	}
}

func (c *Client) setBearer(token string) {
	c.http.SetHeader("Authorization", "Bearer "+token)
}

// do sends one API call, retries included, within the client timeout and
// decodes a 2xx body into out (when non-nil). Every failure to use the
// response is a *errs.CLIError, so CheckResponse reports it unchanged.
func (c *Client) do(method, path string, out any, build func(*resty.Request)) (*resty.Response, error) {
	if err := c.ensureFreshToken(); err != nil {
		return nil, err
	}
	ctx := context.Background()
	if c.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.timeout)
		defer cancel()
	}

	req := c.http.R().SetContext(ctx)
	if build != nil {
		build(req)
	}
	r, err := req.Execute(method, path)
	err = contextErr(ctx, err)
	if errors.Is(err, resty.ErrResponseBodyTooLarge) {
		return r, errs.Wrap(errs.CodeInvalidResponse, fmt.Sprintf("response from %s exceeds %d MiB", path, MaxResponseBytes>>20), err)
	}
	if err != nil || out == nil || r.StatusCode() >= 400 || r.StatusCode() == http.StatusNoContent {
		return r, err
	}
	if err := json.Unmarshal(r.Body(), out); err != nil {
		return r, errs.InvalidResponse(r.StatusCode(), r.Body(), r.Header(), err)
	}
	return r, nil
}

// refuseRedirect stops at the first redirect. Sure's API never redirects,
// and following one would send the API key, bearer token or login password
// to whatever host the Location names.
func refuseRedirect(req *http.Request, _ []*http.Request) error {
	status := 0
	if req.Response != nil {
		status = req.Response.StatusCode
	}
	location := req.URL.Redacted()
	return errs.New(errs.CodeRedirectRefused, fmt.Sprintf("Sure answered with a redirect to %s, which the CLI does not follow; set api_url to the address that serves the API", location)).
		WithDetails(map[string]any{"status": status, "location": location})
}

func withBody(body any) func(*resty.Request) {
	return func(req *resty.Request) { req.SetBody(body) }
}

func (c *Client) Get(path string, out any) (*resty.Response, error) {
	return c.do(http.MethodGet, path, out, nil)
}

func (c *Client) Post(path string, body any, out any) (*resty.Response, error) {
	return c.do(http.MethodPost, path, out, withBody(body))
}

// PostMultipart sends a multipart/form-data POST. fileContentType, when
// non-empty, sets an explicit Content-Type on the file part — required for
// Sure's import endpoints, whose Import::ALLOWED_CSV_MIME_TYPES and
// SureImport::ALLOWED_NDJSON_CONTENT_TYPES allow-lists do exact-match
// `include?` checks that reject the charset suffix resty's default
// auto-detection emits (e.g. "text/plain; charset=utf-8"). Pass "" to keep
// the default detection behavior.
func (c *Client) PostMultipart(path string, fields map[string]string, fileField, filePath, fileContentType string, out any) (*resty.Response, error) {
	if (fileField == "") != (filePath == "") {
		return nil, fmt.Errorf("fileField and filePath must be provided together")
	}
	var file *os.File
	if fileField != "" && fileContentType != "" {
		// SetMultipartField requires an io.Reader.
		f, err := os.Open(filePath)
		if err != nil {
			return nil, fmt.Errorf("open file: %w", err)
		}
		defer f.Close()
		file = f
	}
	return c.do(http.MethodPost, path, out, func(req *resty.Request) {
		if len(fields) > 0 {
			req.SetFormData(fields)
		}
		switch {
		case file != nil:
			req.SetMultipartField(fileField, filepath.Base(filePath), fileContentType, file)
		case fileField != "":
			req.SetFile(fileField, filePath)
		}
	})
}

func (c *Client) Put(path string, body any, out any) (*resty.Response, error) {
	return c.do(http.MethodPut, path, out, withBody(body))
}

func (c *Client) Patch(path string, body any, out any) (*resty.Response, error) {
	return c.do(http.MethodPatch, path, out, withBody(body))
}

func (c *Client) Delete(path string, out any) (*resty.Response, error) {
	return c.do(http.MethodDelete, path, out, nil)
}

// errStalled cancels a download whose body stopped flowing.
var errStalled = errors.New("download stalled")

// GetToFile streams a GET response body to outputPath. The body lands in a
// temp file next to outputPath and is renamed into place only once complete,
// so an error body (e.g. 409 export_not_ready), a captive-portal page or a
// cut-short stream never replaces or masquerades as the downloaded file.
// wantBytes, when positive, is the size the body must have (Sure streams
// exports chunked, without Content-Length). The client timeout bounds the
// wait for headers and each pause in the body, never the whole transfer, and
// a body already flowing is never restarted. Cancelling ctx removes the temp
// file. Every failure is a *errors.CLIError.
func (c *Client) GetToFile(ctx context.Context, path, outputPath string, wantBytes int64) (*resty.Response, error) {
	if err := c.ensureFreshToken(); err != nil {
		return nil, err
	}
	tmp, err := os.CreateTemp(filepath.Dir(outputPath), "."+filepath.Base(outputPath)+".part-*")
	if err != nil {
		return nil, writeFailed(err)
	}
	defer os.Remove(tmp.Name()) // no-op once renamed
	defer tmp.Close()

	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	r, err := c.http.R().SetContext(ctx).SetDoNotParseResponse(true).Get(path)
	if err != nil {
		return r, CheckResponse(r, contextErr(ctx, err))
	}
	body := r.RawBody()
	defer body.Close()

	if r.StatusCode() >= 400 {
		return r, errs.ClassifyHTTPError(r.StatusCode(), readCapped(body), r.Header())
	}
	if mt, _, _ := mime.ParseMediaType(r.Header().Get("Content-Type")); mt == "text/html" {
		return r, errs.InvalidResponse(r.StatusCode(), readCapped(body), r.Header(), errors.New("expected a file, got an HTML page"))
	}

	var src io.Reader = body
	if c.timeout > 0 {
		stall := time.AfterFunc(c.timeout, func() { cancel(errStalled) })
		defer stall.Stop()
		src = &idleReader{r: body, timer: stall, idle: c.timeout}
	}
	dst := &fileWriter{f: tmp}
	n, err := io.Copy(dst, src)
	switch {
	case dst.err != nil:
		return r, writeFailed(dst.err)
	case errors.Is(context.Cause(ctx), errStalled):
		return r, errs.Wrap(errs.CodeTimeout, fmt.Sprintf("download stalled: no data for %s", c.timeout), err)
	case err != nil:
		return r, errs.ClassifyNetworkError(contextErr(ctx, err))
	case wantBytes > 0 && n != wantBytes:
		return r, errs.New(errs.CodeInvalidResponse, fmt.Sprintf("download incomplete: received %d of %d bytes", n, wantBytes)).
			WithDetails(map[string]any{"status": r.StatusCode(), "received_bytes": n, "expected_bytes": wantBytes})
	}
	if err := tmp.Sync(); err != nil {
		return r, writeFailed(err)
	}
	if err := tmp.Close(); err != nil {
		return r, writeFailed(err)
	}
	if err := os.Rename(tmp.Name(), outputPath); err != nil {
		return r, writeFailed(err)
	}
	return r, nil
}

// readCapped reads just enough of an error body for errs to report and cap.
func readCapped(body io.Reader) []byte {
	b, _ := io.ReadAll(io.LimitReader(body, errs.MaxBodyBytes+1))
	return b
}

// contextErr attributes a transport error to ctx when ctx is done: the
// transport may report a cancelled request as a bare "request canceled"
// instead of wrapping ctx.Err().
func contextErr(ctx context.Context, err error) error {
	if err == nil || ctx.Err() == nil || errors.Is(err, ctx.Err()) {
		return err
	}
	return fmt.Errorf("%w: %w", ctx.Err(), err)
}

func writeFailed(err error) *errs.CLIError {
	return errs.Wrap(errs.CodeExportFailed, "cannot write output file: "+err.Error(), err)
}

// fileWriter records a local write error so it is not mistaken for a
// network failure while copying the body.
type fileWriter struct {
	f   *os.File
	err error
}

func (w *fileWriter) Write(p []byte) (int, error) {
	n, err := w.f.Write(p)
	if err != nil {
		w.err = err
	}
	return n, err
}

// idleReader pushes timer back by idle on every read, so the timer fires only
// after idle without any data.
type idleReader struct {
	r     io.Reader
	timer *time.Timer
	idle  time.Duration
}

func (i *idleReader) Read(p []byte) (int, error) {
	n, err := i.r.Read(p)
	i.timer.Reset(i.idle)
	return n, err
}
