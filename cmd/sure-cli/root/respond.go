package root

import (
	"strings"

	"github.com/go-resty/resty/v2"

	errs "github.com/we-promise/sure-cli/internal/errors"
	"github.com/we-promise/sure-cli/internal/output"
)

// maxRespondBodyBytes caps the upstream response body included in an error
// envelope. Sure may echo the original request (including the email or OTP
// for auth flows) in 4xx bodies — keep enough for diagnosis without bulk.
const maxRespondBodyBytes = 1024

// respond is the canonical handler for an API call result. It maps transport
// errors and >=400 HTTP responses to a typed CLIError envelope via
// internal/errors.Classify*, and renders the success payload via output.Print
// otherwise. Centralizing here closes the long-standing bug where every
// print* helper passed 4xx/5xx response bodies through as Envelope.Data
// instead of Envelope.Error.
func respond(r *resty.Response, err error, data any) {
	checkResponse(r, err)
	status := 0
	if r != nil {
		status = r.StatusCode()
	}
	if err := output.Print(format, output.Envelope{Data: data, Meta: &output.Meta{Status: status}}); err != nil {
		output.Fail("output_failed", err.Error(), nil)
	}
}

// checkResponse fails with a typed error envelope on transport error or any
// >=400 HTTP status. On success it returns and the caller can use the
// response. Call sites that need to do typed processing on the body (e.g.
// status_cmd, transactions windowing, insights aggregation) use this instead
// of respond, which always renders.
func checkResponse(r *resty.Response, err error) {
	if err != nil {
		ce := errs.ClassifyNetworkError(err)
		output.Fail(ce.Code, ce.Message, ce.Details)
		return
	}
	if r != nil && r.StatusCode() >= 400 {
		ce := errs.ClassifyHTTPError(r.StatusCode(), r.String())
		output.Fail(ce.Code, ce.Message, mergeErrorDetails(ce.Details, r))
		return
	}
}

// mergeErrorDetails always includes the upstream status and a truncated body
// in error details so agents can introspect 422 validation responses, while
// preserving anything the classifier already attached.
func mergeErrorDetails(classifierDetails map[string]any, r *resty.Response) map[string]any {
	merged := map[string]any{"status": r.StatusCode()}
	if body := strings.TrimSpace(r.String()); body != "" {
		if len(body) > maxRespondBodyBytes {
			body = body[:maxRespondBodyBytes] + "..."
		}
		merged["body"] = body
	}
	for k, v := range classifierDetails {
		merged[k] = v
	}
	return merged
}
