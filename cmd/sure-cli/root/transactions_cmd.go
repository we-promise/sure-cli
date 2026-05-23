package root

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/spf13/cobra"
	"github.com/we-promise/sure-cli/internal/api"
	"github.com/we-promise/sure-cli/internal/output"
)

func newTransactionsCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "transactions", Short: "Transactions"}

	var from, to string
	var startDate, endDate string
	var account, category, merchant string
	var accountID, categoryID, merchantID string
	var typ, search string
	var accountIDs, categoryIDs, merchantIDs, tagIDs []string
	var minAmount, maxAmount string
	var page, perPage int

	list := &cobra.Command{
		Use:   "list",
		Short: "List transactions",
		Run: func(cmd *cobra.Command, args []string) {
			client := api.New()

			q := url.Values{}
			if startDate == "" {
				startDate = from
			}
			if endDate == "" {
				endDate = to
			}
			if accountID == "" {
				accountID = account
			}
			if categoryID == "" {
				categoryID = category
			}
			if merchantID == "" {
				merchantID = merchant
			}
			if startDate != "" {
				q.Set("start_date", startDate)
			}
			if endDate != "" {
				q.Set("end_date", endDate)
			}
			if accountID != "" {
				q.Set("account_id", accountID)
			}
			if categoryID != "" {
				q.Set("category_id", categoryID)
			}
			if merchantID != "" {
				q.Set("merchant_id", merchantID)
			}
			if minAmount != "" {
				q.Set("min_amount", minAmount)
			}
			if maxAmount != "" {
				q.Set("max_amount", maxAmount)
			}
			if typ != "" {
				q.Set("type", typ)
			}
			if search != "" {
				q.Set("search", search)
			}
			for _, id := range splitFlagValues(accountIDs) {
				q.Add("account_ids[]", id)
			}
			for _, id := range splitFlagValues(categoryIDs) {
				q.Add("category_ids[]", id)
			}
			for _, id := range splitFlagValues(merchantIDs) {
				q.Add("merchant_ids[]", id)
			}
			for _, id := range splitFlagValues(tagIDs) {
				q.Add("tag_ids[]", id)
			}
			if page > 0 {
				q.Set("page", fmt.Sprintf("%d", page))
			}
			if perPage > 0 {
				q.Set("per_page", fmt.Sprintf("%d", perPage))
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
			_ = output.Print(format, output.Envelope{Data: res, Meta: &output.Meta{Status: r.StatusCode()}})
		},
	}

	list.Flags().StringVar(&from, "from", "", "start date (YYYY-MM-DD)")
	list.Flags().StringVar(&to, "to", "", "end date (YYYY-MM-DD)")
	list.Flags().StringVar(&startDate, "start-date", "", "start date (YYYY-MM-DD, maps to start_date)")
	list.Flags().StringVar(&endDate, "end-date", "", "end date (YYYY-MM-DD, maps to end_date)")
	list.Flags().StringVar(&account, "account", "", "account id (alias for --account-id)")
	list.Flags().StringVar(&category, "category", "", "category id (alias for --category-id)")
	list.Flags().StringVar(&merchant, "merchant", "", "merchant id (alias for --merchant-id)")
	list.Flags().StringVar(&accountID, "account-id", "", "account id")
	list.Flags().StringVar(&categoryID, "category-id", "", "category id")
	list.Flags().StringVar(&merchantID, "merchant-id", "", "merchant id")
	list.Flags().StringVar(&minAmount, "min-amount", "", "minimum amount")
	list.Flags().StringVar(&maxAmount, "max-amount", "", "maximum amount")
	list.Flags().StringVar(&typ, "type", "", "transaction type: income|expense")
	list.Flags().StringVar(&search, "search", "", "search name, notes, or merchant")
	list.Flags().StringSliceVar(&accountIDs, "account-ids", nil, "account ids (repeat or comma-separated)")
	list.Flags().StringSliceVar(&categoryIDs, "category-ids", nil, "category ids (repeat or comma-separated)")
	list.Flags().StringSliceVar(&merchantIDs, "merchant-ids", nil, "merchant ids (repeat or comma-separated)")
	list.Flags().StringSliceVar(&tagIDs, "tag-ids", nil, "tag ids (repeat or comma-separated)")
	list.Flags().IntVar(&page, "page", 1, "page number")
	list.Flags().IntVar(&perPage, "per-page", 25, "items per page (maps to per_page)")
	cmd.AddCommand(list)

	cmd.AddCommand(&cobra.Command{
		Use:   "show <id>",
		Short: "Show transaction",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			client := api.New()
			var res any
			path := fmt.Sprintf("/api/v1/transactions/%s", url.PathEscape(args[0]))
			r, err := client.Get(path, &res)
			if err != nil {
				output.Fail("request_failed", err.Error(), nil)
			}
			_ = output.Print(format, output.Envelope{Data: res, Meta: &output.Meta{Status: r.StatusCode()}})
		},
	})

	cmd.AddCommand(newTransactionsCreateCmd())
	cmd.AddCommand(newTransactionsUpdateCmd())
	cmd.AddCommand(newTransactionsDeleteCmd())

	return cmd
}

func splitFlagValues(values []string) []string {
	var out []string
	for _, value := range values {
		for _, part := range strings.Split(value, ",") {
			part = strings.TrimSpace(part)
			if part != "" {
				out = append(out, part)
			}
		}
	}
	return out
}
