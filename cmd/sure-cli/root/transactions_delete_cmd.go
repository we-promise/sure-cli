package root

import (
	"fmt"
	"net/url"

	"github.com/spf13/cobra"

	"github.com/we-promise/sure-cli/internal/api"
	"github.com/we-promise/sure-cli/internal/output"
)

type txDeleteOpts struct {
	Apply bool
}

func newTransactionsDeleteCmd() *cobra.Command {
	var o txDeleteOpts

	cmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a transaction (default dry-run; use --apply to execute)",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			id := args[0]
			path := fmt.Sprintf("/api/v1/transactions/%s", url.PathEscape(id))

			if !o.Apply {
				_ = output.Print(format, output.Envelope{Data: map[string]any{
					"dry_run": true,
					"request": map[string]any{
						"method": "DELETE",
						"path":   path,
					},
				}})
				return
			}

			client := api.New()
			var res any
			r, err := client.Delete(path, &res)
			respond(r, err, res)
		},
	}

	cmd.Flags().BoolVar(&o.Apply, "apply", false, "execute the delete (otherwise dry-run)")
	return cmd
}
