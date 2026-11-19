package root

import (
	"github.com/go-resty/resty/v2"

	"github.com/we-promise/sure-cli/internal/api"
	"github.com/we-promise/sure-cli/internal/output"
)

// respond is the canonical handler for an API call result. It maps transport
// errors and >=400 HTTP responses to a typed CLIError envelope via
// api.CheckResponse, and renders the success payload via output.Print
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
	if ce := api.CheckResponse(r, err); ce != nil {
		output.Fail(ce.Code, ce.Message, ce.Details)
	}
}
