package root

import (
	"fmt"
	"net/url"

	"github.com/spf13/cobra"
)

func newHoldingsCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "holdings", Short: "Investment holdings (requires Sure API support)"}
	cmd.AddCommand(newHoldingsListCmd())
	cmd.AddCommand(&cobra.Command{
		Use:   "show <id>",
		Short: "Show investment holding",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			printGet(fmt.Sprintf("/api/v1/holdings/%s", args[0]))
		},
	})
	return cmd
}

func newHoldingsListCmd() *cobra.Command {
	var page, perPage int
	var accountID, date, startDate, endDate, securityID string
	var accountIDs []string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List investment holdings",
		Run: func(cmd *cobra.Command, args []string) {
			q := url.Values{}
			addPagingQuery(q, page, perPage)
			if accountID != "" {
				q.Set("account_id", accountID)
			}
			addRepeatedQuery(q, "account_ids", accountIDs)
			if date != "" {
				q.Set("date", date)
			}
			if startDate != "" {
				q.Set("start_date", startDate)
			}
			if endDate != "" {
				q.Set("end_date", endDate)
			}
			if securityID != "" {
				q.Set("security_id", securityID)
			}
			printGet(pathWithQuery("/api/v1/holdings", q))
		},
	}
	addPagingFlags(cmd, &page, &perPage)
	cmd.Flags().StringVar(&accountID, "account-id", "", "account id")
	cmd.Flags().StringSliceVar(&accountIDs, "account-ids", nil, "account ids (repeat or comma-separated)")
	cmd.Flags().StringVar(&date, "date", "", "exact holding date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&startDate, "start-date", "", "start date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&endDate, "end-date", "", "end date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&securityID, "security-id", "", "security id")
	return cmd
}
