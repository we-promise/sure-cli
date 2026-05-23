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
			if page > 0 {
				q.Set("page", fmt.Sprintf("%d", page))
			}
			if perPage > 0 {
				q.Set("per_page", fmt.Sprintf("%d", perPage))
			}
			path := "/api/v1/family_exports"
			if encoded := q.Encode(); encoded != "" {
				path = path + "?" + encoded
			}
			printFamilyExportGet(path)
		},
	}
	list.Flags().IntVar(&page, "page", 1, "page number")
	list.Flags().IntVar(&perPage, "per-page", 25, "items per page (maps to per_page)")
	cmd.AddCommand(list)

	cmd.AddCommand(&cobra.Command{
		Use:   "show <id>",
		Short: "Show family export",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			printFamilyExportGet(fmt.Sprintf("/api/v1/family_exports/%s", url.PathEscape(args[0])))
		},
	})

	var apply bool
	create := &cobra.Command{
		Use:   "create",
		Short: "Queue a family export (default dry-run; use --apply to execute)",
		Run: func(cmd *cobra.Command, args []string) {
			path := "/api/v1/family_exports"
			if !apply {
				printFamilyExportDryRun("POST", path, nil)
				return
			}
			client := api.New()
			var res any
			r, err := client.Post(path, map[string]any{}, &res)
			respond(r, err, res)
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

func printFamilyExportGet(path string) {
	client := api.New()
	var res any
	r, err := client.Get(path, &res)
	respond(r, err, res)
}

func printFamilyExportDryRun(method, path string, body any) {
	request := map[string]any{
		"method": method,
		"path":   path,
	}
	if body != nil {
		request["body"] = body
	}
	if err := output.Print(format, output.Envelope{Data: map[string]any{
		"dry_run": true,
		"request": request,
	}}); err != nil {
		output.Fail("output_failed", err.Error(), nil)
	}
}
