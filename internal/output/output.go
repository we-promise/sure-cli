package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// toStderr is set when the command's output file is stdout itself (see
// EnvelopesToStderr).
var toStderr bool

// EnvelopesToStderr sends every later envelope, success or failure, to
// stderr. A command writing its data to stdout (--out /dev/stdout) calls it
// so the data stays one document.
func EnvelopesToStderr() { toStderr = true }

// envelopes is where envelopes and tables go, looked up on each write.
func envelopes() io.Writer {
	if toStderr {
		return os.Stderr
	}
	return os.Stdout
}

type Envelope struct {
	Data  any    `json:"data,omitempty"`
	Error *Error `json:"error,omitempty"`
	Meta  *Meta  `json:"meta,omitempty"`
}

type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

func PrintJSON(v any) error {
	enc := json.NewEncoder(envelopes())
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func Print(format string, env Envelope) error {
	switch format {
	case "table":
		if ok := PrintTable(env); ok {
			return nil
		}
		// fallback
		return PrintJSON(env)
	default:
		return PrintJSON(env)
	}
}

func Fail(code, message string, details any) {
	_ = PrintJSON(Envelope{Error: &Error{Code: code, Message: message, Details: details}})
	fmt.Fprintln(os.Stderr, Printable(message))
	os.Exit(1)
}
