package api

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
)

// idempotentMethods are the only methods the client retries. Sure can fail
// with 5xx after persisting a write (e.g. imports#create answers 500
// publish_failed with the created import_id), so retrying a POST/PATCH would
// duplicate data.
var idempotentMethods = map[string]bool{
	http.MethodGet:     true,
	http.MethodHead:    true,
	http.MethodPut:     true,
	http.MethodDelete:  true,
	http.MethodOptions: true,
}

// shouldRetry retries idempotent requests on network errors and 5xx, and on
// 429 only when Retry-After asks for a wait no longer than maxWait: Sure sets
// it to the seconds left in the rate-limit window, and retrying sooner would
// just spend the remaining attempts on guaranteed 429s.
func shouldRetry(r *resty.Response, err error, maxWait time.Duration) bool {
	if r == nil || r.Request == nil || !idempotentMethods[r.Request.Method] {
		return false
	}
	if err != nil {
		return true
	}
	if r.StatusCode() == http.StatusTooManyRequests {
		d, ok := retryAfter(r.Header())
		return ok && d <= maxWait
	}
	return r.StatusCode() >= 500
}

// retryAfter parses a delta-seconds Retry-After header (the only form Sure
// emits). ok is false when the header is absent or not a non-negative integer.
func retryAfter(h http.Header) (time.Duration, bool) {
	secs, err := strconv.Atoi(strings.TrimSpace(h.Get("Retry-After")))
	if err != nil || secs < 0 {
		return 0, false
	}
	return time.Duration(secs) * time.Second, true
}
