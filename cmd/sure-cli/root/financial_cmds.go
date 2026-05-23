package root

import (
	"fmt"
	"net/url"
	"time"

	"github.com/spf13/cobra"

	"github.com/we-promise/sure-cli/internal/output"
)

func newBalanceSheetCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "balance-sheet", Short: "Balance sheet"}
	cmd.AddCommand(&cobra.Command{
		Use:   "show",
		Short: "Show balance sheet",
		Run: func(cmd *cobra.Command, args []string) {
			printGet("/api/v1/balance_sheet")
		},
	})
	return cmd
}

func newBalancesCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "balances", Short: "Balance history"}

	var page, perPage int
	var accountID, currency, startDate, endDate string
	list := &cobra.Command{
		Use:   "list",
		Short: "List balance history records",
		Run: func(cmd *cobra.Command, args []string) {
			q := url.Values{}
			addPagingQuery(q, page, perPage)
			if accountID != "" {
				q.Set("account_id", accountID)
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
			printGet(pathWithQuery("/api/v1/balances", q))
		},
	}
	addPagingFlags(list, &page, &perPage)
	list.Flags().StringVar(&accountID, "account-id", "", "account id")
	list.Flags().StringVar(&currency, "currency", "", "currency")
	list.Flags().StringVar(&startDate, "start-date", "", "start date (YYYY-MM-DD)")
	list.Flags().StringVar(&endDate, "end-date", "", "end date (YYYY-MM-DD)")
	cmd.AddCommand(list)

	cmd.AddCommand(&cobra.Command{
		Use:   "show <id>",
		Short: "Show balance history record",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			printGet(fmt.Sprintf("/api/v1/balances/%s", url.PathEscape(args[0])))
		},
	})
	return cmd
}

func newFamilySettingsCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "family-settings", Short: "Family settings"}
	cmd.AddCommand(&cobra.Command{
		Use:   "show",
		Short: "Show family settings",
		Run: func(cmd *cobra.Command, args []string) {
			printGet("/api/v1/family_settings")
		},
	})
	return cmd
}

type valuationCreateOpts struct {
	AccountID string
	Amount    string
	Date      string
	Notes     string
	Upsert    bool
	Apply     bool
}

type valuationUpdateOpts struct {
	Amount string
	Date   string
	Notes  string
	Apply  bool
}

func newValuationsCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "valuations", Short: "Valuations"}

	var page, perPage int
	var accountID, startDate, endDate string
	list := &cobra.Command{
		Use:   "list",
		Short: "List valuations",
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
			printGet(pathWithQuery("/api/v1/valuations", q))
		},
	}
	addPagingFlags(list, &page, &perPage)
	list.Flags().StringVar(&accountID, "account-id", "", "account id")
	list.Flags().StringVar(&startDate, "start-date", "", "start date (YYYY-MM-DD)")
	list.Flags().StringVar(&endDate, "end-date", "", "end date (YYYY-MM-DD)")
	cmd.AddCommand(list)

	cmd.AddCommand(&cobra.Command{
		Use:   "show <id>",
		Short: "Show valuation",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			printGet(fmt.Sprintf("/api/v1/valuations/%s", url.PathEscape(args[0])))
		},
	})

	cmd.AddCommand(newValuationsCreateCmd())
	cmd.AddCommand(newValuationsUpdateCmd())
	return cmd
}

func newValuationsCreateCmd() *cobra.Command {
	var o valuationCreateOpts
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create valuation (default dry-run; use --apply to execute)",
		Run: func(cmd *cobra.Command, args []string) {
			payload, err := buildValuationCreatePayload(o)
			if err != nil {
				output.Fail("validation_failed", err.Error(), nil)
				return
			}
			dispatchWrite(o.Apply, "POST", "/api/v1/valuations", payload)
		},
	}
	cmd.Flags().StringVar(&o.AccountID, "account-id", "", "account id (required)")
	cmd.Flags().StringVar(&o.Amount, "amount", "", "valuation amount (required)")
	cmd.Flags().StringVar(&o.Date, "date", time.Now().Format("2006-01-02"), "date YYYY-MM-DD")
	cmd.Flags().StringVar(&o.Notes, "notes", "", "notes")
	cmd.Flags().BoolVar(&o.Upsert, "upsert", false, "request upsert response semantics")
	cmd.Flags().BoolVar(&o.Apply, "apply", false, "execute the create (otherwise dry-run)")
	return cmd
}

func newValuationsUpdateCmd() *cobra.Command {
	var o valuationUpdateOpts
	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update valuation (default dry-run; use --apply to execute)",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			payload, err := buildValuationUpdatePayload(o)
			if err != nil {
				output.Fail("validation_failed", err.Error(), nil)
				return
			}
			dispatchWrite(o.Apply, "PATCH", fmt.Sprintf("/api/v1/valuations/%s", url.PathEscape(args[0])), payload)
		},
	}
	cmd.Flags().StringVar(&o.Amount, "amount", "", "valuation amount")
	cmd.Flags().StringVar(&o.Date, "date", "", "date YYYY-MM-DD")
	cmd.Flags().StringVar(&o.Notes, "notes", "", "notes")
	cmd.Flags().BoolVar(&o.Apply, "apply", false, "execute the update (otherwise dry-run)")
	return cmd
}

func buildValuationCreatePayload(o valuationCreateOpts) (map[string]any, error) {
	if o.AccountID == "" {
		return nil, fmt.Errorf("account-id is required")
	}
	if o.Amount == "" {
		return nil, fmt.Errorf("amount is required")
	}
	if _, err := time.Parse("2006-01-02", o.Date); err != nil {
		return nil, fmt.Errorf("invalid date (expected YYYY-MM-DD): %w", err)
	}
	valuation := map[string]any{
		"account_id": o.AccountID,
		"amount":     o.Amount,
		"date":       o.Date,
	}
	if o.Notes != "" {
		valuation["notes"] = o.Notes
	}
	payload := map[string]any{"valuation": valuation}
	if o.Upsert {
		payload["upsert"] = true
	}
	return payload, nil
}

func buildValuationUpdatePayload(o valuationUpdateOpts) (map[string]any, error) {
	if o.Amount == "" && o.Date == "" && o.Notes == "" {
		return nil, fmt.Errorf("no fields provided to update")
	}
	if (o.Amount == "") != (o.Date == "") {
		return nil, fmt.Errorf("amount and date must both be provided when updating valuation amount")
	}
	valuation := map[string]any{}
	if o.Amount != "" {
		if _, err := time.Parse("2006-01-02", o.Date); err != nil {
			return nil, fmt.Errorf("invalid date (expected YYYY-MM-DD): %w", err)
		}
		valuation["amount"] = o.Amount
		valuation["date"] = o.Date
	}
	if o.Notes != "" {
		valuation["notes"] = o.Notes
	}
	return map[string]any{"valuation": valuation}, nil
}
