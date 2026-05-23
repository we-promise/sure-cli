package root

import (
	"fmt"
	"net/url"

	"github.com/spf13/cobra"
	"github.com/we-promise/sure-cli/internal/api"
)

func newAccountsCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "accounts", Short: "Accounts"}

	var page, perPage int
	list := &cobra.Command{
		Use:   "list",
		Short: "List accounts",
		Run: func(cmd *cobra.Command, args []string) {
			client := api.New()
			path := "/api/v1/accounts"
			if page > 0 || perPage > 0 {
				q := url.Values{}
				if page > 0 {
					q.Set("page", fmt.Sprintf("%d", page))
				}
				if perPage > 0 {
					q.Set("per_page", fmt.Sprintf("%d", perPage))
				}
				path = path + "?" + q.Encode()
			}
			var res any
			r, err := client.Get(path, &res)
			respond(r, err, res)
		},
	}
	list.Flags().IntVar(&page, "page", 1, "page number")
	list.Flags().IntVar(&perPage, "per-page", 25, "items per page (maps to per_page)")
	cmd.AddCommand(list)

	cmd.AddCommand(&cobra.Command{
		Use:   "show <id>",
		Short: "Show account",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			client := api.New()
			var res any
			path := fmt.Sprintf("/api/v1/accounts/%s", url.PathEscape(args[0]))
			r, err := client.Get(path, &res)
			respond(r, err, res)
		},
	})

	return cmd
}
