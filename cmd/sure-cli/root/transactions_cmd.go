package root

import (
	"cmp"
	"fmt"
	"net/url"
	"strings"

	"github.com/spf13/cobra"

	"github.com/we-promise/sure-cli/internal/api"
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
			start, end := cmp.Or(startDate, from), cmp.Or(endDate, to)
			// Errors name the flag the user typed, --from/--to included.
			startFlag, endFlag := "start-date", "end-date"
			if startDate == "" && from != "" {
				startFlag = "from"
			}
			if endDate == "" && to != "" {
				endFlag = "to"
			}
			failValidation(checkTransactionsListFilters(startFlag, start, endFlag, end, minAmount, maxAmount, typ))
			q := url.Values{}
			for key, value := range map[string]string{
				"start_date": start, "end_date": end,
				"account_id": cmp.Or(accountID, account), "category_id": cmp.Or(categoryID, category), "merchant_id": cmp.Or(merchantID, merchant),
				"min_amount": minAmount, "max_amount": maxAmount, "type": strings.ToLower(typ), "search": search,
			} {
				if value != "" {
					q.Set(key, value)
				}
			}
			addRepeatedQuery(q, "account_ids", splitFlagValues(accountIDs))
			addRepeatedQuery(q, "category_ids", splitFlagValues(categoryIDs))
			addRepeatedQuery(q, "merchant_ids", splitFlagValues(merchantIDs))
			addRepeatedQuery(q, "tag_ids", splitFlagValues(tagIDs))
			addPagingQuery(q, page, perPage)
			printGet(pathWithQuery("/api/v1/transactions", q))
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
	list.Flags().StringVar(&minAmount, "min-amount", "", "minimum amount, in Sure's sign: an expense is positive, an income negative")
	list.Flags().StringVar(&maxAmount, "max-amount", "", "maximum amount, in Sure's sign: an expense is positive, an income negative")
	list.Flags().StringVar(&typ, "type", "", "transaction type: income|expense")
	list.Flags().StringVar(&search, "search", "", "search name, notes, or merchant")
	list.Flags().StringSliceVar(&accountIDs, "account-ids", nil, "account ids (repeat or comma-separated)")
	list.Flags().StringSliceVar(&categoryIDs, "category-ids", nil, "category ids (repeat or comma-separated)")
	list.Flags().StringSliceVar(&merchantIDs, "merchant-ids", nil, "merchant ids (repeat or comma-separated)")
	list.Flags().StringSliceVar(&tagIDs, "tag-ids", nil, "tag ids (repeat or comma-separated)")
	addPagingFlags(list, &page, &perPage)
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
			respond(r, err, res)
		},
	})

	cmd.AddCommand(newTransactionsCreateCmd())
	cmd.AddCommand(newTransactionsUpdateCmd())
	cmd.AddCommand(newTransactionsDeleteCmd())

	return cmd
}

// checkTransactionsListFilters rejects filters Sure would misread: its
// apply_filters parses dates with Date.parse (a bad one is a 500, 05/01/2026
// is 5 January), amounts with to_f (abc is 0) and ignores an unknown type.
func checkTransactionsListFilters(startFlag, start, endFlag, end, minAmount, maxAmount, typ string) error {
	s, err := parseOptionalISODate(startFlag, start)
	if err != nil {
		return err
	}
	e, err := parseOptionalISODate(endFlag, end)
	if err != nil {
		return err
	}
	if !s.IsZero() && !e.IsZero() && s.After(e) {
		return fmt.Errorf("--%s must be on or before --%s", startFlag, endFlag)
	}
	if err := checkOptionalDecimal("min-amount", minAmount, moneyColumn); err != nil {
		return err
	}
	if err := checkOptionalDecimal("max-amount", maxAmount, moneyColumn); err != nil {
		return err
	}
	if t := strings.ToLower(typ); t != "" && t != "income" && t != "expense" {
		return fmt.Errorf("--type must be income or expense, got %q", typ)
	}
	return nil
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
