package root

import (
	"path/filepath"
	"strings"
)

// uploadContentTypes maps file extensions to the parameter-free Content-Type
// sent on multipart file parts. Sure's upload endpoints do exact-match
// `include?` checks against their allow-lists (Import::ALLOWED_CSV_MIME_TYPES,
// SureImport::ALLOWED_NDJSON_CONTENT_TYPES,
// AccountStatement::ALLOWED_EXTENSION_CONTENT_TYPES), so the charset-suffixed
// value resty would auto-detect ("text/plain; charset=utf-8") is rejected.
var uploadContentTypes = map[string]string{
	".csv":    "text/csv",
	".ndjson": "application/x-ndjson",
	".json":   "application/json",
	".pdf":    "application/pdf",
	".xlsx":   "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
}

// mimeForUploadFile picks the Content-Type for a multipart file part from its
// extension (case-insensitive). Returning "" lets resty fall back to
// detection for unknown extensions.
func mimeForUploadFile(filePath string) string {
	return uploadContentTypes[strings.ToLower(filepath.Ext(filePath))]
}
