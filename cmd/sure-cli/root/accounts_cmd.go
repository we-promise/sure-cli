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
	var includeDisabled, all bool
	list := &cobra.Command{
		Use:   "list",
		Short: "List accounts",
		Run: func(cmd *cobra.Command, args []string) {
			client := api.New()
			if all {
				accounts, r, err := api.FetchAllAccounts(client, includeDisabled)
				if accounts == nil {
					accounts = []map[string]any{} // render [], never null
				}
				checkResponse(r, err)
				printEnvelope(map[string]any{"accounts": accounts}, nil) // several pages: no meta.status
				return
			}
			q := api.AccountsQuery(includeDisabled)
			addPagingQuery(q, page, perPage)
			var res any
			r, err := client.Get(pathWithQuery("/api/v1/accounts", q), &res)
			respond(r, err, res)
		},
	}
	addPagingFlags(list, &page, &perPage)
	list.Flags().BoolVar(&includeDisabled, "include-disabled", false, "include disabled accounts (maps to include_disabled)")
	list.Flags().BoolVar(&all, "all", false, "fetch every page (ignores --page/--per-page)")
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
