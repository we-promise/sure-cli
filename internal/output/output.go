package output

import (
	"encoding/json"
	"fmt"
	"os"
)

type Envelope struct {
	Data  any    `json:"data,omitempty"`
	Error *Error `json:"error,omitempty"`
	Meta  any    `json:"meta,omitempty"`
}

type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

func PrintJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func Fail(code, message string, details any) {
	_ = PrintJSON(Envelope{Error: &Error{Code: code, Message: message, Details: details}})
	fmt.Fprintln(os.Stderr, message)
	os.Exit(1)
}
