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
			printGet(fmt.Sprintf("/api/v1/family_exports/%s", args[0]))
		},
	})

	var apply bool
	create := &cobra.Command{
		Use:   "create",
		Short: "Queue a family export (default dry-run; use --apply to execute)",
		Run: func(cmd *cobra.Command, args []string) {
			path := "/api/v1/family_exports"
			if !apply {
				printDryRun("POST", path, nil)
				return
			}
			printPost(path, map[string]any{})
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
			}
			client := api.New()
			path := fmt.Sprintf("/api/v1/family_exports/%s/download", args[0])
			r, err := client.GetToFile(path, outFile)
			if err != nil {
				output.Fail("request_failed", err.Error(), nil)
			}
			if err := output.Print(format, output.Envelope{Data: map[string]any{
				"file": outFile,
			}, Meta: &output.Meta{Status: r.StatusCode()}}); err != nil {
				output.Fail("output_failed", err.Error(), nil)
			}
		},
	}
	download.Flags().StringVar(&outFile, "out", "", "output file path (required)")
	cmd.AddCommand(download)

	return cmd
}
