package root

import (
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
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
			printGet(fmt.Sprintf("/api/v1/securities/%s", url.PathEscape(args[0])))
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
			printGet(fmt.Sprintf("/api/v1/security_prices/%s", url.PathEscape(args[0])))
		},
	})
	cmd.AddCommand(newSecurityPricesCreateCmd())
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
	PaymentURL         string
	Autopay            string
	Notes              string
	Apply              bool
}

type recurringUpdateOpts struct {
	Status             string
	ExpectedDayOfMonth string
	NextExpectedDate   string
	PaymentURL         string
	Autopay            string
	Notes              string
	// Empty holds the flags given as "" (see emptyFlags).
	Empty map[string]bool
	Apply bool
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
	list.Flags().StringVar(&status, "status", "", "status filter: suggested|active|paused|inactive|ended")
	list.Flags().StringVar(&accountID, "account-id", "", "account id filter")
	cmd.AddCommand(list)

	cmd.AddCommand(&cobra.Command{
		Use:   "show <id>",
		Short: "Show recurring transaction",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			printGet(fmt.Sprintf("/api/v1/recurring_transactions/%s", url.PathEscape(args[0])))
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
		Long: `Create recurring transaction (default dry-run; use --apply to execute).

Amounts follow Sure's entry convention: positive is a bill, negative an
income.`,
		Run: func(cmd *cobra.Command, args []string) {
			payload, err := buildRecurringCreatePayload(o)
			failValidation(err)
			dispatchWrite(o.Apply, "POST", "/api/v1/recurring_transactions", payload)
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
	addRecurringStatusFlag(cmd, &o.Status)
	cmd.Flags().StringVar(&o.OccurrenceCount, "occurrence-count", "", "occurrence count")
	cmd.Flags().StringVar(&o.Manual, "manual", "", "manual flag")
	cmd.Flags().StringVar(&o.ExpectedAmountMin, "expected-amount-min", "", "expected amount min")
	cmd.Flags().StringVar(&o.ExpectedAmountMax, "expected-amount-max", "", "expected amount max")
	cmd.Flags().StringVar(&o.ExpectedAmountAvg, "expected-amount-avg", "", "expected amount avg")
	addRecurringBillFlags(cmd, &o.PaymentURL, &o.Autopay, &o.Notes)
	cmd.Flags().BoolVar(&o.Apply, "apply", false, "execute the create (otherwise dry-run)")
	return cmd
}

func newRecurringTransactionsUpdateCmd() *cobra.Command {
	var o recurringUpdateOpts
	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update recurring transaction (default dry-run; use --apply to execute)",
		Long: `Update recurring transaction (default dry-run; use --apply to execute).

An empty --notes or --payment-url ("") clears the field.`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			o.Empty = emptyFlags(cmd, "notes", "payment-url")
			payload, err := buildRecurringUpdatePayload(o)
			failValidation(err)
			dispatchWrite(o.Apply, "PATCH", fmt.Sprintf("/api/v1/recurring_transactions/%s", url.PathEscape(args[0])), payload)
		},
	}
	addRecurringStatusFlag(cmd, &o.Status)
	cmd.Flags().StringVar(&o.ExpectedDayOfMonth, "expected-day-of-month", "", "expected day of month")
	cmd.Flags().StringVar(&o.NextExpectedDate, "next-expected-date", "", "next expected date YYYY-MM-DD")
	addRecurringBillFlags(cmd, &o.PaymentURL, &o.Autopay, &o.Notes)
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
			dispatchWrite(apply, "DELETE", fmt.Sprintf("/api/v1/recurring_transactions/%s", url.PathEscape(args[0])), nil)
		},
	}
	cmd.Flags().BoolVar(&apply, "apply", false, "execute the delete (otherwise dry-run)")
	return cmd
}

func buildRecurringCreatePayload(o recurringCreateOpts) (map[string]any, error) {
	// RecurringTransaction validates amount, currency and
	// expected_day_of_month; the controller adds both dates and fills in
	// none of them.
	for _, f := range []struct{ name, value string }{
		{"--amount", o.Amount}, {"--currency", o.Currency}, {"--expected-day-of-month", o.ExpectedDayOfMonth},
		{"--last-occurrence-date", o.LastOccurrenceDate}, {"--next-expected-date", o.NextExpectedDate},
	} {
		if f.value == "" {
			return nil, fmt.Errorf("%s is required", f.name)
		}
	}
	// amount follows the entry convention (an income series is negative).
	for _, f := range []struct{ name, value string }{
		{"amount", o.Amount}, {"expected-amount-min", o.ExpectedAmountMin},
		{"expected-amount-max", o.ExpectedAmountMax}, {"expected-amount-avg", o.ExpectedAmountAvg},
	} {
		if err := checkOptionalDecimal(f.name, f.value, moneyColumn); err != nil {
			return nil, err
		}
	}
	if err := checkRecurringSchedule(o.ExpectedDayOfMonth, o.LastOccurrenceDate, o.NextExpectedDate); err != nil {
		return nil, err
	}
	if err := validateRecurringStatus(o.Status); err != nil {
		return nil, err
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
	if err := addRecurringBillFields(recurring, o.PaymentURL, o.Autopay, o.Notes); err != nil {
		return nil, err
	}
	return map[string]any{"recurring_transaction": recurring}, nil
}

func buildRecurringUpdatePayload(o recurringUpdateOpts) (map[string]any, error) {
	if err := checkRecurringSchedule(o.ExpectedDayOfMonth, "", o.NextExpectedDate); err != nil {
		return nil, err
	}
	if err := validateRecurringStatus(o.Status); err != nil {
		return nil, err
	}
	recurring := map[string]any{}
	addAny(recurring, "status", o.Status)
	addAny(recurring, "expected_day_of_month", o.ExpectedDayOfMonth)
	addAny(recurring, "next_expected_date", o.NextExpectedDate)
	if err := addRecurringBillFields(recurring, o.PaymentURL, o.Autopay, o.Notes); err != nil {
		return nil, err
	}
	addCleared(recurring, o.Empty, map[string]string{"notes": "notes", "payment-url": "payment_url"})
	if len(recurring) == 0 {
		return nil, fmt.Errorf("no fields provided to update")
	}
	return map[string]any{"recurring_transaction": recurring}, nil
}

// checkRecurringSchedule checks the schedule flags that are set: the day of
// month Sure accepts (1-31) and ISO dates, which Rails would otherwise parse
// loosely (05/01/2026 is 5 January).
func checkRecurringSchedule(dayOfMonth, lastOccurrence, nextExpected string) error {
	if dayOfMonth != "" {
		if day, err := strconv.Atoi(dayOfMonth); err != nil || day < 1 || day > 31 {
			return fmt.Errorf("--expected-day-of-month must be a whole number from 1 to 31, got %q", dayOfMonth)
		}
	}
	if _, err := parseOptionalISODate("last-occurrence-date", lastOccurrence); err != nil {
		return err
	}
	_, err := parseOptionalISODate("next-expected-date", nextExpected)
	return err
}

// recurringWritableStatuses mirrors Sure's RecurringTransactionsController
// WRITABLE_STATUSES: the API writes only these two; suggested, paused and
// ended belong to detection and the web UI.
var recurringWritableStatuses = []string{"active", "inactive"}

func addRecurringStatusFlag(cmd *cobra.Command, status *string) {
	cmd.Flags().StringVar(status, "status", "", "status: "+strings.Join(recurringWritableStatuses, "|"))
}

func validateRecurringStatus(status string) error {
	if status == "" || slices.Contains(recurringWritableStatuses, status) {
		return nil
	}
	return fmt.Errorf("--status must be one of: %s (got %q)", strings.Join(recurringWritableStatuses, "|"), status)
}

func addRecurringBillFlags(cmd *cobra.Command, paymentURL, autopay, notes *string) {
	cmd.Flags().StringVar(paymentURL, "payment-url", "", "http(s) URL where the bill is paid")
	cmd.Flags().StringVar(autopay, "autopay", "", "true|false: whether the bill is paid automatically")
	cmd.Flags().StringVar(notes, "notes", "", "notes")
}

func addRecurringBillFields(recurring map[string]any, paymentURL, autopay, notes string) error {
	addAny(recurring, "payment_url", paymentURL)
	addAny(recurring, "notes", notes)
	switch autopay {
	case "":
	case "true", "false":
		recurring["autopay"] = autopay == "true"
	default:
		return fmt.Errorf("--autopay must be true or false (got %q)", autopay)
	}
	return nil
}
