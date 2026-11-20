package output

// Meta is an optional envelope metadata section intended for agents.
//
// Schema: optional $id of the JSON schema that `data` conforms to.
// Status: HTTP status code (when known).
// Warnings: what the command could not do and worked around, each with the
// error code that caused it; data is still valid.
type Meta struct {
	Schema   string  `json:"schema,omitempty"`
	Status   int     `json:"status,omitempty"`
	Warnings []Error `json:"warnings,omitempty"`
}
