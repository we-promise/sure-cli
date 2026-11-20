package root

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/we-promise/sure-cli/internal/api"
)

func newFamilyExportsCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "family-exports", Short: "Family exports"}

	var page, perPage int
	list := &cobra.Command{
		Use:   "list",
		Short: "List family exports",
		Run: func(cmd *cobra.Command, args []string) {
			q := url.Values{}
			addPagingQuery(q, page, perPage)
			printGet(pathWithQuery("/api/v1/family_exports", q))
		},
	}
	addPagingFlags(list, &page, &perPage)
	cmd.AddCommand(list)

	cmd.AddCommand(&cobra.Command{
		Use:   "show <id>",
		Short: "Show family export",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			printGet(fmt.Sprintf("/api/v1/family_exports/%s", url.PathEscape(args[0])))
		},
	})

	var apply bool
	create := &cobra.Command{
		Use:   "create",
		Short: "Queue a family export (default dry-run; use --apply to execute)",
		Run: func(cmd *cobra.Command, args []string) {
			// Upstream Api::V1::FamilyExportsController#create ignores the body
			// (it queues a job per current_resource_owner.family); send {} on
			// apply and pass nil for dry-run to keep the previous envelope
			// shape (no body key under request).
			if !apply {
				dispatchWrite(apply, "POST", "/api/v1/family_exports", nil)
				return
			}
			dispatchWrite(true, "POST", "/api/v1/family_exports", map[string]any{})
		},
	}
	create.Flags().BoolVar(&apply, "apply", false, "execute the create (otherwise dry-run)")
	cmd.AddCommand(create)

	var outFile string
	download := &cobra.Command{
		Use:   "download <id>",
		Short: "Download a completed family export",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			if outFile == "" {
				failValidation(errors.New("--out is required"))
			}
			// Ctrl-C cancels the transfer instead of killing the process, so
			// the partial temp file next to --out is removed.
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			client := api.New()
			exportPath := fmt.Sprintf("/api/v1/family_exports/%s", url.PathEscape(args[0]))
			// Sure streams the file without Content-Length; its byte_size is
			// what tells a complete download from a cut-short one.
			var export struct {
				Data struct {
					File struct {
						ByteSize int64 `json:"byte_size"`
					} `json:"file"`
				} `json:"data"`
			}
			checkResponse(client.Get(exportPath, &export))
			r, err := client.GetToFile(ctx, exportPath+"/download", outFile, export.Data.File.ByteSize)
			// Download returns the file path as the data payload on success.
			// On a non-2xx (e.g. 409 export_not_ready) or an incomplete body
			// GetToFile leaves --out untouched and returns a typed error.
			respond(r, err, map[string]any{"file": outFile})
		},
	}
	download.Flags().StringVar(&outFile, "out", "", "output file path (required)")
	cmd.AddCommand(download)

	return cmd
}
