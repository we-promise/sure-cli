package root

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/spf13/cobra"
	"github.com/we-promise/sure-cli/internal/api"
	"github.com/we-promise/sure-cli/internal/output"
)

func newTradesCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "trades", Short: "Trades"}

	var from, to string
	var startDate, endDate string
	var account, accountID string
	var accountIDs []string
	var page, perPage int

	list := &cobra.Command{
		Use:   "list",
		Short: "List trades",
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
			if startDate != "" {
				q.Set("start_date", startDate)
			}
			if endDate != "" {
				q.Set("end_date", endDate)
			}
			if accountID != "" {
				q.Set("account_id", accountID)
			}
			for _, id := range splitTradeFlagValues(accountIDs) {
				q.Add("account_ids[]", id)
			}
			if page > 0 {
				q.Set("page", fmt.Sprintf("%d", page))
			}
			if perPage > 0 {
				q.Set("per_page", fmt.Sprintf("%d", perPage))
			}

			u := url.URL{Path: "/api/v1/trades", RawQuery: q.Encode()}
			path := u.String()

			var res any
			r, err := client.Get(path, &res)
			if err != nil {
				output.Fail("request_failed", err.Error(), nil)
			}
			if err := output.Print(format, output.Envelope{Data: res, Meta: &output.Meta{Status: r.StatusCode()}}); err != nil {
				output.Fail("output_failed", err.Error(), nil)
			}
		},
	}

	list.Flags().StringVar(&from, "from", "", "start date (YYYY-MM-DD)")
	list.Flags().StringVar(&to, "to", "", "end date (YYYY-MM-DD)")
	list.Flags().StringVar(&startDate, "start-date", "", "start date (YYYY-MM-DD, maps to start_date)")
	list.Flags().StringVar(&endDate, "end-date", "", "end date (YYYY-MM-DD, maps to end_date)")
	list.Flags().StringVar(&account, "account", "", "account id (alias for --account-id)")
	list.Flags().StringVar(&accountID, "account-id", "", "account id")
	list.Flags().StringSliceVar(&accountIDs, "account-ids", nil, "account ids (repeat or comma-separated)")
	list.Flags().IntVar(&page, "page", 1, "page number")
	list.Flags().IntVar(&perPage, "per-page", 25, "items per page (maps to per_page)")
	cmd.AddCommand(list)

	cmd.AddCommand(&cobra.Command{
		Use:   "show <id>",
		Short: "Show trade",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			client := api.New()
			var res any
			path := fmt.Sprintf("/api/v1/trades/%s", url.PathEscape(args[0]))
			r, err := client.Get(path, &res)
			if err != nil {
				output.Fail("request_failed", err.Error(), nil)
			}
			if err := output.Print(format, output.Envelope{Data: res, Meta: &output.Meta{Status: r.StatusCode()}}); err != nil {
				output.Fail("output_failed", err.Error(), nil)
			}
		},
	})

	return cmd
}

func splitTradeFlagValues(values []string) []string {
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
