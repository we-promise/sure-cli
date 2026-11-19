package api

import (
	"errors"

	"github.com/go-resty/resty/v2"

	errs "github.com/we-promise/sure-cli/internal/errors"
)

// CheckResponse turns a request result into a typed CLIError, or nil on a
// <400 response. A CLIError already in err's chain is returned unchanged;
// any other transport error is classified as a network error.
func CheckResponse(r *resty.Response, err error) *errs.CLIError {
	if err != nil {
		var ce *errs.CLIError
		if errors.As(err, &ce) {
			return ce
		}
		return errs.ClassifyNetworkError(err)
	}
	if r != nil && r.StatusCode() >= 400 {
		return errs.ClassifyHTTPError(r.StatusCode(), r.Body(), r.Header())
	}
	return nil
}
