package root

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-resty/resty/v2"

	"github.com/we-promise/sure-cli/internal/api"
)

// upload is a POST whose content travels either as the multipart part "file"
// read from FilePath, or as raw_file_content in a JSON body. Fields go along
// as form fields or JSON members.
type upload struct {
	Fields   map[string]string
	FilePath string
	Raw      string
}

func (u upload) send(client *api.Client, path string, out any) (*resty.Response, error) {
	if u.FilePath == "" {
		return client.Post(path, u.jsonBody(), out)
	}
	return client.PostMultipart(path, u.Fields, "file", u.FilePath, mimeForUploadFile(u.FilePath), out)
}

func (u upload) jsonBody() map[string]any {
	body := make(map[string]any, len(u.Fields)+1)
	for k, v := range u.Fields {
		body[k] = v
	}
	if u.Raw != "" {
		body["raw_file_content"] = u.Raw
	}
	return body
}

// dryRunBody describes the planned request; a file appears as its path, so
// the dry run never embeds the document.
func (u upload) dryRunBody() map[string]any {
	body := u.jsonBody()
	if u.FilePath != "" {
		body["file"] = u.FilePath
	}
	return body
}

// checkUploadContent requires exactly one of --file and --rawFlag, and a
// file to be a regular file.
func checkUploadContent(file, raw, rawFlag string) error {
	switch {
	case file == "" && raw == "":
		return fmt.Errorf("file or %s is required", rawFlag)
	case file != "" && raw != "":
		return fmt.Errorf("provide only one of file or %s", rawFlag)
	case file != "":
		_, err := regularFile(file)
		return err
	}
	return nil
}

// regularFile stats path and fails unless it is an accessible regular file.
func regularFile(path string) (os.FileInfo, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("file not accessible: %w", err)
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("file must be a regular file")
	}
	return info, nil
}

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
