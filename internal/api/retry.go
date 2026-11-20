package api

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"

	errs "github.com/we-promise/sure-cli/internal/errors"
)

// retriedMethods are the only methods the client retries. Sure can fail
// with 5xx after persisting a write (e.g. imports#create answers 500
// publish_failed with the created import_id), so retrying a POST/PATCH would
// duplicate data. DELETE is left out too: when a proxy loses the answer to a
// committed delete, the retry reports not_found for a delete that succeeded.
var retriedMethods = map[string]bool{
	http.MethodGet:     true,
	http.MethodHead:    true,
	http.MethodPut:     true,
	http.MethodOptions: true,
}

// shouldRetry retries retriedMethods on network errors and 5xx, and on
// 429 only when Retry-After asks for a wait no longer than maxWait: Sure sets
// it to the seconds left in the rate-limit window, and retrying sooner would
// just spend the remaining attempts on guaranteed 429s.
func shouldRetry(r *resty.Response, err error, maxWait time.Duration) bool {
	if r == nil || r.Request == nil || !retriedMethods[r.Request.Method] {
		return false
	}
	if err != nil {
		// An oversized body, a refused redirect or an expired/cancelled call
		// fails the same way on every attempt.
		var ce *errs.CLIError
		return !errors.As(err, &ce) && !errors.Is(err, resty.ErrResponseBodyTooLarge) && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded)
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

// retryWait is the wait before the retry that follows r: Sure's Retry-After
// when it sends one, else min doubled per attempt made, capped at max. It is
// deterministic (no jitter) so the retry condition can tell whether the
// retry fits in what is left of the call's deadline.
func retryWait(r *resty.Response, base, ceiling time.Duration) time.Duration {
	wait, ok := retryAfter(r.Header())
	if !ok {
		wait = base
		for i := 1; i < r.Request.Attempt && wait < ceiling; i++ {
			wait *= 2
		}
	}
	return max(base, min(wait, ceiling))
}

// fitsDeadline reports whether waiting wait before retrying r leaves time
// before the call's deadline. Without it the deadline would expire during
// the wait, and the call would end in a timeout instead of the answer Sure
// last gave (e.g. a 503 with its maintenance message).
func fitsDeadline(r *resty.Response, wait time.Duration) bool {
	deadline, ok := r.Request.Context().Deadline()
	return !ok || time.Until(deadline) > wait
}
