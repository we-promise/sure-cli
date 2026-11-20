package root

import (
	"github.com/go-resty/resty/v2"

	"github.com/we-promise/sure-cli/internal/api"
	errs "github.com/we-promise/sure-cli/internal/errors"
	"github.com/we-promise/sure-cli/internal/output"
)

// respond renders one Sure response: a typed error envelope for a transport
// error or an HTTP status >= 400, else data with meta.status set to the
// response's status.
func respond(r *resty.Response, err error, data any) {
	checkResponse(r, err)
	status := 0
	if r != nil {
		status = r.StatusCode()
	}
	printEnvelope(data, &output.Meta{Status: status})
}

// checkResponse fails with a typed error envelope on transport error or any
// >=400 HTTP status. On success it returns and the caller can use the
// response. Commands that process the body before printing it use this
// instead of respond.
func checkResponse(r *resty.Response, err error) {
	if ce := api.CheckResponse(r, err); ce != nil {
		fail(ce)
	}
}

// warnings holds what the command worked around so far (see warn).
var warnings []output.Error

// warn records ce as worked around: the command goes on, and the envelope
// it prints lists ce under meta.warnings.
func warn(ce *errs.CLIError) {
	warnings = append(warnings, output.Error{Code: ce.Code, Message: ce.Message, Details: ce.Details})
}

// printEnvelope prints data in the --format chosen, with the warnings
// recorded so far. meta.status is set only when data is a single Sure
// response; output computed from several calls or locally leaves it out.
func printEnvelope(data any, meta *output.Meta) {
	if len(warnings) > 0 {
		if meta == nil {
			meta = &output.Meta{}
		}
		meta.Warnings, warnings = append(meta.Warnings, warnings...), nil
	}
	if err := output.Print(format, output.Envelope{Data: data, Meta: meta}); err != nil {
		fail(errs.Wrap(errs.CodeOutputFailed, err.Error(), err))
	}
}

// fail prints ce as the error envelope and exits 1.
func fail(ce *errs.CLIError) {
	output.Fail(ce.Code, ce.Message, ce.Details)
}

// failValidation fails with validation_failed when err is not nil.
func failValidation(err error) {
	if err != nil {
		output.Fail(errs.CodeValidation, err.Error(), nil)
	}
}
