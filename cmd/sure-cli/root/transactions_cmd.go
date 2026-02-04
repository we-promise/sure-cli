package root

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/dgilperez/sure-cli/internal/api"
	"github.com/dgilperez/sure-cli/internal/output"
	"github.com/spf13/cobra"
)

func newTransactionsCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "transactions", Short: "Transactions"}

	var from, to string
	var account, category, merchant string
	var page, perPage int
	var limit int

	list := &cobra.Command{
		Use:   "list",
		Short: "List transactions",
		Run: func(cmd *cobra.Command, args []string) {
			client := api.New()

			q := url.Values{}
			if from != "" {
				q.Set("from", from)
			}
			if to != "" {
				q.Set("to", to)
			}
			if account != "" {
				q.Set("account", account)
			}
			if category != "" {
				q.Set("category", category)
			}
			if merchant != "" {
				q.Set("merchant", merchant)
			}
			if page > 0 {
				q.Set("page", fmt.Sprintf("%d", page))
			}
			if perPage > 0 {
				q.Set("per_page", fmt.Sprintf("%d", perPage))
			}
			if limit > 0 {
				q.Set("limit", fmt.Sprintf("%d", limit))
			}

			path := "/api/v1/transactions"
			if enc := strings.TrimPrefix(q.Encode(), ""); enc != "" {
				path = path + "?" + enc
			}

			var res any
			r, err := client.Get(path, &res)
			if err != nil {
				output.Fail("request_failed", err.Error(), nil)
			}
			_ = output.Print(format, output.Envelope{Data: res, Meta: map[string]any{"status": r.StatusCode()}})
		},
	}

	list.Flags().StringVar(&from, "from", "", "start date (YYYY-MM-DD)")
	list.Flags().StringVar(&to, "to", "", "end date (YYYY-MM-DD)")
	list.Flags().StringVar(&account, "account", "", "account id")
	list.Flags().StringVar(&category, "category", "", "category id")
	list.Flags().StringVar(&merchant, "merchant", "", "merchant id")
	list.Flags().IntVar(&page, "page", 1, "page number")
	list.Flags().IntVar(&perPage, "per-page", 25, "items per page (maps to per_page)")
	list.Flags().IntVar(&limit, "limit", 50, "max results")
	cmd.AddCommand(list)

	cmd.AddCommand(&cobra.Command{
		Use:   "show <id>",
		Short: "Show transaction",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			client := api.New()
			var res any
			path := fmt.Sprintf("/api/v1/transactions/%s", args[0])
			r, err := client.Get(path, &res)
			if err != nil {
				output.Fail("request_failed", err.Error(), nil)
			}
			_ = output.Print(format, output.Envelope{Data: res, Meta: map[string]any{"status": r.StatusCode()}})
		},
	})

	return cmd
}
