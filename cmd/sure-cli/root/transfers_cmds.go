package root

import (
	"fmt"
	"net/url"

	"github.com/spf13/cobra"
)

func newTransfersCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "transfers", Short: "Transfers (categorized transfers, payments, loan payments)"}

	var status, accountID, startDate, endDate string
	var page, perPage int

	list := &cobra.Command{
		Use:   "list",
		Short: "List transfers",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			q := url.Values{}
			addPagingQuery(q, page, perPage)
			if status != "" {
				q.Set("status", status)
			}
			if accountID != "" {
				q.Set("account_id", accountID)
			}
			if startDate != "" {
				q.Set("start_date", startDate)
			}
			if endDate != "" {
				q.Set("end_date", endDate)
			}
			printGet(pathWithQuery("/api/v1/transfers", q))
		},
	}
	addPagingFlags(list, &page, &perPage)
	list.Flags().StringVar(&status, "status", "", "filter by status (e.g. pending, confirmed)")
	list.Flags().StringVar(&accountID, "account-id", "", "filter by account id (UUID)")
	list.Flags().StringVar(&startDate, "start-date", "", "earliest entry date (YYYY-MM-DD)")
	list.Flags().StringVar(&endDate, "end-date", "", "latest entry date (YYYY-MM-DD)")
	cmd.AddCommand(list)

	cmd.AddCommand(&cobra.Command{
		Use:   "show <id>",
		Short: "Show transfer",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			printGet(fmt.Sprintf("/api/v1/transfers/%s", url.PathEscape(args[0])))
		},
	})

	return cmd
}

func newRejectedTransfersCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "rejected-transfers", Short: "Rejected transfer suggestions"}

	var accountID, startDate, endDate string
	var page, perPage int

	list := &cobra.Command{
		Use:   "list",
		Short: "List rejected transfers",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			q := url.Values{}
			addPagingQuery(q, page, perPage)
			if accountID != "" {
				q.Set("account_id", accountID)
			}
			if startDate != "" {
				q.Set("start_date", startDate)
			}
			if endDate != "" {
				q.Set("end_date", endDate)
			}
			printGet(pathWithQuery("/api/v1/rejected_transfers", q))
		},
	}
	addPagingFlags(list, &page, &perPage)
	list.Flags().StringVar(&accountID, "account-id", "", "filter by account id (UUID)")
	list.Flags().StringVar(&startDate, "start-date", "", "earliest entry date (YYYY-MM-DD)")
	list.Flags().StringVar(&endDate, "end-date", "", "latest entry date (YYYY-MM-DD)")
	cmd.AddCommand(list)

	cmd.AddCommand(&cobra.Command{
		Use:   "show <id>",
		Short: "Show rejected transfer",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			printGet(fmt.Sprintf("/api/v1/rejected_transfers/%s", url.PathEscape(args[0])))
		},
	})

	return cmd
}
