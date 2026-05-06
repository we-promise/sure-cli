package root

import (
	"fmt"
	"net/url"

	"github.com/spf13/cobra"
	"github.com/we-promise/sure-cli/internal/output"
)

func newSecuritiesCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "securities", Short: "Securities"}

	var page, perPage int
	var ticker, exchangeOperatingMIC, kind, offline string
	list := &cobra.Command{
		Use:   "list",
		Short: "List securities",
		Run: func(cmd *cobra.Command, args []string) {
			q := url.Values{}
			addPagingQuery(q, page, perPage)
			if ticker != "" {
				q.Set("ticker", ticker)
			}
			if exchangeOperatingMIC != "" {
				q.Set("exchange_operating_mic", exchangeOperatingMIC)
			}
			if kind != "" {
				q.Set("kind", kind)
			}
			if offline != "" {
				q.Set("offline", offline)
			}
			printGet(pathWithQuery("/api/v1/securities", q))
		},
	}
	addPagingFlags(list, &page, &perPage)
	list.Flags().StringVar(&ticker, "ticker", "", "ticker filter")
	list.Flags().StringVar(&exchangeOperatingMIC, "exchange-operating-mic", "", "exchange operating MIC filter")
	list.Flags().StringVar(&kind, "kind", "", "security kind filter")
	list.Flags().StringVar(&offline, "offline", "", "offline filter: true|false")
	cmd.AddCommand(list)

	cmd.AddCommand(&cobra.Command{
		Use:   "show <id>",
		Short: "Show security",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			printGet(fmt.Sprintf("/api/v1/securities/%s", args[0]))
		},
	})
	return cmd
}

func newSecurityPricesCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "security-prices", Short: "Security prices"}

	var page, perPage int
	var securityID, currency, startDate, endDate, provisional string
	list := &cobra.Command{
		Use:   "list",
		Short: "List security price history",
		Run: func(cmd *cobra.Command, args []string) {
			q := url.Values{}
			addPagingQuery(q, page, perPage)
			if securityID != "" {
				q.Set("security_id", securityID)
			}
			if currency != "" {
				q.Set("currency", currency)
			}
			if startDate != "" {
				q.Set("start_date", startDate)
			}
			if endDate != "" {
				q.Set("end_date", endDate)
			}
			if provisional != "" {
				q.Set("provisional", provisional)
			}
			printGet(pathWithQuery("/api/v1/security_prices", q))
		},
	}
	addPagingFlags(list, &page, &perPage)
	list.Flags().StringVar(&securityID, "security-id", "", "security id")
	list.Flags().StringVar(&currency, "currency", "", "currency")
	list.Flags().StringVar(&startDate, "start-date", "", "start date (YYYY-MM-DD)")
	list.Flags().StringVar(&endDate, "end-date", "", "end date (YYYY-MM-DD)")
	list.Flags().StringVar(&provisional, "provisional", "", "provisional filter: true|false")
	cmd.AddCommand(list)

	cmd.AddCommand(&cobra.Command{
		Use:   "show <id>",
		Short: "Show security price",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			printGet(fmt.Sprintf("/api/v1/security_prices/%s", args[0]))
		},
	})
	return cmd
}

type recurringCreateOpts struct {
	Name               string
	Amount             string
	Currency           string
	AccountID          string
	MerchantID         string
	ExpectedDayOfMonth string
	LastOccurrenceDate string
	NextExpectedDate   string
	Status             string
	OccurrenceCount    string
	Manual             string
	ExpectedAmountMin  string
	ExpectedAmountMax  string
	ExpectedAmountAvg  string
	Apply              bool
}

type recurringUpdateOpts struct {
	Status             string
	ExpectedDayOfMonth string
	NextExpectedDate   string
	Apply              bool
}

func newRecurringTransactionsCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "recurring-transactions", Short: "Recurring transactions"}

	var page, perPage int
	var status, accountID string
	list := &cobra.Command{
		Use:   "list",
		Short: "List recurring transactions",
		Run: func(cmd *cobra.Command, args []string) {
			q := url.Values{}
			addPagingQuery(q, page, perPage)
			if status != "" {
				q.Set("status", status)
			}
			if accountID != "" {
				q.Set("account_id", accountID)
			}
			printGet(pathWithQuery("/api/v1/recurring_transactions", q))
		},
	}
	addPagingFlags(list, &page, &perPage)
	list.Flags().StringVar(&status, "status", "", "status filter")
	list.Flags().StringVar(&accountID, "account-id", "", "account id filter")
	cmd.AddCommand(list)

	cmd.AddCommand(&cobra.Command{
		Use:   "show <id>",
		Short: "Show recurring transaction",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			printGet(fmt.Sprintf("/api/v1/recurring_transactions/%s", args[0]))
		},
	})
	cmd.AddCommand(newRecurringTransactionsCreateCmd())
	cmd.AddCommand(newRecurringTransactionsUpdateCmd())
	cmd.AddCommand(newRecurringTransactionsDeleteCmd())
	return cmd
}

func newRecurringTransactionsCreateCmd() *cobra.Command {
	var o recurringCreateOpts
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create recurring transaction (default dry-run; use --apply to execute)",
		Run: func(cmd *cobra.Command, args []string) {
			payload, err := buildRecurringCreatePayload(o)
			if err != nil {
				output.Fail("validation_failed", err.Error(), nil)
			}
			path := "/api/v1/recurring_transactions"
			if !o.Apply {
				printDryRun("POST", path, payload)
				return
			}
			printPost(path, payload)
		},
	}
	cmd.Flags().StringVar(&o.Name, "name", "", "name")
	cmd.Flags().StringVar(&o.Amount, "amount", "", "amount")
	cmd.Flags().StringVar(&o.Currency, "currency", "", "currency")
	cmd.Flags().StringVar(&o.AccountID, "account-id", "", "account id")
	cmd.Flags().StringVar(&o.MerchantID, "merchant-id", "", "merchant id")
	cmd.Flags().StringVar(&o.ExpectedDayOfMonth, "expected-day-of-month", "", "expected day of month")
	cmd.Flags().StringVar(&o.LastOccurrenceDate, "last-occurrence-date", "", "last occurrence date YYYY-MM-DD")
	cmd.Flags().StringVar(&o.NextExpectedDate, "next-expected-date", "", "next expected date YYYY-MM-DD")
	cmd.Flags().StringVar(&o.Status, "status", "", "status")
	cmd.Flags().StringVar(&o.OccurrenceCount, "occurrence-count", "", "occurrence count")
	cmd.Flags().StringVar(&o.Manual, "manual", "", "manual flag")
	cmd.Flags().StringVar(&o.ExpectedAmountMin, "expected-amount-min", "", "expected amount min")
	cmd.Flags().StringVar(&o.ExpectedAmountMax, "expected-amount-max", "", "expected amount max")
	cmd.Flags().StringVar(&o.ExpectedAmountAvg, "expected-amount-avg", "", "expected amount avg")
	cmd.Flags().BoolVar(&o.Apply, "apply", false, "execute the create (otherwise dry-run)")
	return cmd
}

func newRecurringTransactionsUpdateCmd() *cobra.Command {
	var o recurringUpdateOpts
	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update recurring transaction (default dry-run; use --apply to execute)",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			payload, err := buildRecurringUpdatePayload(o)
			if err != nil {
				output.Fail("validation_failed", err.Error(), nil)
			}
			path := fmt.Sprintf("/api/v1/recurring_transactions/%s", args[0])
			if !o.Apply {
				printDryRun("PATCH", path, payload)
				return
			}
			printPatch(path, payload)
		},
	}
	cmd.Flags().StringVar(&o.Status, "status", "", "status")
	cmd.Flags().StringVar(&o.ExpectedDayOfMonth, "expected-day-of-month", "", "expected day of month")
	cmd.Flags().StringVar(&o.NextExpectedDate, "next-expected-date", "", "next expected date YYYY-MM-DD")
	cmd.Flags().BoolVar(&o.Apply, "apply", false, "execute the update (otherwise dry-run)")
	return cmd
}

func newRecurringTransactionsDeleteCmd() *cobra.Command {
	var apply bool
	cmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete recurring transaction (default dry-run; use --apply to execute)",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			path := fmt.Sprintf("/api/v1/recurring_transactions/%s", args[0])
			if !apply {
				printDryRun("DELETE", path, nil)
				return
			}
			printDelete(path)
		},
	}
	cmd.Flags().BoolVar(&apply, "apply", false, "execute the delete (otherwise dry-run)")
	return cmd
}

func buildRecurringCreatePayload(o recurringCreateOpts) (map[string]any, error) {
	if o.LastOccurrenceDate == "" {
		return nil, fmt.Errorf("last-occurrence-date is required")
	}
	if o.NextExpectedDate == "" {
		return nil, fmt.Errorf("next-expected-date is required")
	}
	recurring := map[string]any{}
	addAny(recurring, "name", o.Name)
	addAny(recurring, "amount", o.Amount)
	addAny(recurring, "currency", o.Currency)
	addAny(recurring, "account_id", o.AccountID)
	addAny(recurring, "merchant_id", o.MerchantID)
	addAny(recurring, "expected_day_of_month", o.ExpectedDayOfMonth)
	addAny(recurring, "last_occurrence_date", o.LastOccurrenceDate)
	addAny(recurring, "next_expected_date", o.NextExpectedDate)
	addAny(recurring, "status", o.Status)
	addAny(recurring, "occurrence_count", o.OccurrenceCount)
	addAny(recurring, "manual", o.Manual)
	addAny(recurring, "expected_amount_min", o.ExpectedAmountMin)
	addAny(recurring, "expected_amount_max", o.ExpectedAmountMax)
	addAny(recurring, "expected_amount_avg", o.ExpectedAmountAvg)
	return map[string]any{"recurring_transaction": recurring}, nil
}

func buildRecurringUpdatePayload(o recurringUpdateOpts) (map[string]any, error) {
	recurring := map[string]any{}
	addAny(recurring, "status", o.Status)
	addAny(recurring, "expected_day_of_month", o.ExpectedDayOfMonth)
	addAny(recurring, "next_expected_date", o.NextExpectedDate)
	if len(recurring) == 0 {
		return nil, fmt.Errorf("no fields provided to update")
	}
	return map[string]any{"recurring_transaction": recurring}, nil
}
