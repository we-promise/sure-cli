package root

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-resty/resty/v2"
	"github.com/spf13/cobra"
	"github.com/we-promise/sure-cli/internal/api"
)

// newImportSessionsCmd wires the chunked Sure NDJSON import flow:
//
//	create  -> POST   /api/v1/import_sessions
//	show    -> GET    /api/v1/import_sessions/:id
//	add-chunk -> POST /api/v1/import_sessions/:id/chunks
//	publish -> POST   /api/v1/import_sessions/:id/publish
//
// Unlike `imports` (CSV with column mapping), import_sessions uploads a Sure
// NDJSON export in one or more sequenced chunks and then publishes the whole
// session for processing. All params are top-level (the controller reads
// params[:type], params[:sequence], ... directly — no wrapping key).
func newImportSessionsCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "import-sessions", Short: "Import sessions (chunked Sure NDJSON import)"}

	cmd.AddCommand(&cobra.Command{
		Use:   "show <id>",
		Short: "Show import session",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			printGet(fmt.Sprintf("/api/v1/import_sessions/%s", url.PathEscape(args[0])))
		},
	})

	cmd.AddCommand(newImportSessionsCreateCmd())
	cmd.AddCommand(newImportSessionsAddChunkCmd())
	cmd.AddCommand(newImportSessionsPublishCmd())

	return cmd
}

type importSessionCreateOpts struct {
	Type            string
	ClientSessionID string
	ExpectedChunks  int
	Apply           bool
}

// buildImportSessionCreatePayload builds the top-level create body. The
// controller does `params[:type].to_s` and treats blank as an empty import
// type, so we reject an empty type client-side for fast feedback.
func buildImportSessionCreatePayload(o importSessionCreateOpts) (map[string]any, error) {
	t := strings.TrimSpace(o.Type)
	if t == "" {
		return nil, errors.New("type is required (e.g. SureImport)")
	}
	if o.ExpectedChunks < 0 {
		return nil, fmt.Errorf("expected-chunks must be >= 0, got %d", o.ExpectedChunks)
	}
	body := map[string]any{"type": t}
	if o.ClientSessionID != "" {
		body["client_session_id"] = o.ClientSessionID
	}
	if o.ExpectedChunks > 0 {
		body["expected_chunks"] = o.ExpectedChunks
	}
	return body, nil
}

func newImportSessionsCreateCmd() *cobra.Command {
	var o importSessionCreateOpts
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create an import session (default dry-run; use --apply to execute)",
		Run: func(cmd *cobra.Command, args []string) {
			payload, err := buildImportSessionCreatePayload(o)
			if err != nil {
				failValidation(err)
			}
			dispatchWrite(o.Apply, "POST", "/api/v1/import_sessions", payload)
		},
	}
	cmd.Flags().StringVar(&o.Type, "type", "SureImport", "import type")
	cmd.Flags().StringVar(&o.ClientSessionID, "client-session-id", "", "idempotency key; reuses an existing session for the same id")
	cmd.Flags().IntVar(&o.ExpectedChunks, "expected-chunks", 0, "number of chunks you plan to upload (optional)")
	cmd.Flags().BoolVar(&o.Apply, "apply", false, "execute the create (otherwise dry-run)")
	return cmd
}

type chunkCreateOpts struct {
	Sequence      int
	ClientChunkID string
	File          string
	RawContent    string
	Apply         bool
}

// chunkCreate is the resolved upload plan: either a multipart file part or a
// JSON body carrying raw_file_content. Exactly one is set.
type chunkCreate struct {
	Multipart bool
	// multipart fields (string values), used when Multipart is true
	Fields   map[string]string
	FilePath string
	// JSON body, used when Multipart is false (raw content path)
	JSONBody map[string]any
}

// buildChunkCreate validates and resolves a chunk upload. The controller
// requires `sequence` and exactly one content source — a Sure NDJSON `file`
// (multipart) or `raw_file_content` (string). Extension is checked
// client-side because the controller only accepts NDJSON/JSON content types.
func buildChunkCreate(o chunkCreateOpts) (chunkCreate, error) {
	if o.Sequence < 1 {
		return chunkCreate{}, fmt.Errorf("sequence is required and must be >= 1, got %d", o.Sequence)
	}
	if o.File == "" && o.RawContent == "" {
		return chunkCreate{}, errors.New("file or raw-content is required")
	}
	if o.File != "" && o.RawContent != "" {
		return chunkCreate{}, errors.New("provide only one of file or raw-content")
	}

	if o.File != "" {
		ext := strings.ToLower(filepath.Ext(o.File))
		if ext != ".ndjson" && ext != ".json" {
			return chunkCreate{}, fmt.Errorf("file must be a Sure NDJSON file (.ndjson or .json), got %q", filepath.Ext(o.File))
		}
		info, err := os.Stat(o.File)
		if err != nil {
			return chunkCreate{}, fmt.Errorf("file not accessible: %w", err)
		}
		if info.IsDir() {
			return chunkCreate{}, errors.New("file must be a regular file")
		}
		fields := map[string]string{"sequence": strconv.Itoa(o.Sequence)}
		if o.ClientChunkID != "" {
			fields["client_chunk_id"] = o.ClientChunkID
		}
		return chunkCreate{Multipart: true, Fields: fields, FilePath: o.File}, nil
	}

	body := map[string]any{
		"sequence":         o.Sequence,
		"raw_file_content": o.RawContent,
	}
	if o.ClientChunkID != "" {
		body["client_chunk_id"] = o.ClientChunkID
	}
	return chunkCreate{Multipart: false, JSONBody: body}, nil
}

func newImportSessionsAddChunkCmd() *cobra.Command {
	var o chunkCreateOpts
	cmd := &cobra.Command{
		Use:   "add-chunk <session_id>",
		Short: "Upload a chunk to an import session (default dry-run; use --apply to execute)",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			plan, err := buildChunkCreate(o)
			if err != nil {
				failValidation(err)
			}
			path := fmt.Sprintf("/api/v1/import_sessions/%s/chunks", url.PathEscape(args[0]))

			if !o.Apply {
				printDryRun("POST", path, chunkDryRunBody(plan))
				return
			}

			client := api.New()
			var res any
			var r *resty.Response
			if plan.Multipart {
				r, err = client.PostMultipart(path, plan.Fields, "file", plan.FilePath, mimeForImportFile(plan.FilePath), &res)
			} else {
				r, err = client.Post(path, plan.JSONBody, &res)
			}
			respond(r, err, res)
		},
	}
	cmd.Flags().IntVar(&o.Sequence, "sequence", 0, "chunk sequence number (required, >= 1)")
	cmd.Flags().StringVar(&o.ClientChunkID, "client-chunk-id", "", "idempotency key for this chunk (optional)")
	cmd.Flags().StringVar(&o.File, "file", "", "path to a Sure NDJSON file (.ndjson/.json)")
	cmd.Flags().StringVar(&o.RawContent, "raw-content", "", "raw Sure NDJSON content (alternative to --file)")
	cmd.Flags().BoolVar(&o.Apply, "apply", false, "execute the upload (otherwise dry-run)")
	return cmd
}

// chunkDryRunBody renders the planned request for a dry-run without leaking a
// whole NDJSON file into the envelope: the multipart file is shown as a path
// reference, raw content is passed through.
func chunkDryRunBody(plan chunkCreate) map[string]any {
	if plan.Multipart {
		body := map[string]any{"file": plan.FilePath}
		for k, v := range plan.Fields {
			body[k] = v
		}
		return body
	}
	return plan.JSONBody
}

func newImportSessionsPublishCmd() *cobra.Command {
	var apply bool
	cmd := &cobra.Command{
		Use:   "publish <id>",
		Short: "Publish an import session for processing (default dry-run; use --apply to execute)",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			path := fmt.Sprintf("/api/v1/import_sessions/%s/publish", url.PathEscape(args[0]))
			dispatchWrite(apply, "POST", path, nil)
		},
	}
	cmd.Flags().BoolVar(&apply, "apply", false, "execute the publish (otherwise dry-run)")
	return cmd
}
