package root

import (
	"fmt"
	"net/url"

	"github.com/spf13/cobra"

	"github.com/we-promise/sure-cli/internal/api"
	"github.com/we-promise/sure-cli/internal/output"
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
				output.Fail("validation_failed", "out is required", nil)
				return
			}
			client := api.New()
			path := fmt.Sprintf("/api/v1/family_exports/%s/download", url.PathEscape(args[0]))
			r, err := client.GetToFile(path, outFile)
			// Download returns the file path as the data payload on success;
			// error/status routing matches every other GET via respond.
			respond(r, err, map[string]any{"file": outFile})
		},
	}
	download.Flags().StringVar(&outFile, "out", "", "output file path (required)")
	cmd.AddCommand(download)

	return cmd
}
