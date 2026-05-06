package root

import (
	"fmt"
	"net/url"

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
			addRepeatedQuery(q, "account_ids", accountIDs)
			addPagingQuery(q, page, perPage)

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
	addPagingFlags(list, &page, &perPage)
	cmd.AddCommand(list)

	cmd.AddCommand(&cobra.Command{
		Use:   "show <id>",
		Short: "Show trade",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			client := api.New()
			var res any
			path := fmt.Sprintf("/api/v1/trades/%s", args[0])
			r, err := client.Get(path, &res)
			if err != nil {
				output.Fail("request_failed", err.Error(), nil)
			}
			if err := output.Print(format, output.Envelope{Data: res, Meta: &output.Meta{Status: r.StatusCode()}}); err != nil {
				output.Fail("output_failed", err.Error(), nil)
			}
		},
	})

	cmd.AddCommand(newTradesCreateCmd())
	cmd.AddCommand(newTradesUpdateCmd())
	cmd.AddCommand(newTradesDeleteCmd())

	return cmd
}

type tradeCreateOpts struct {
	AccountID               string
	Date                    string
	Type                    string
	Qty                     string
	Price                   string
	Currency                string
	SecurityID              string
	Ticker                  string
	ManualTicker            string
	InvestmentActivityLabel string
	CategoryID              string
	Apply                   bool
}

type tradeUpdateOpts struct {
	Name                    string
	Date                    string
	Amount                  string
	Currency                string
	Notes                   string
	Nature                  string
	Type                    string
	Qty                     string
	Price                   string
	InvestmentActivityLabel string
	CategoryID              string
	Apply                   bool
}

func newTradesCreateCmd() *cobra.Command {
	var o tradeCreateOpts
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create trade (default dry-run; use --apply to execute)",
		Run: func(cmd *cobra.Command, args []string) {
			payload, err := buildTradeCreatePayload(o)
			if err != nil {
				output.Fail("validation_failed", err.Error(), nil)
			}
			path := "/api/v1/trades"
			if !o.Apply {
				printDryRun("POST", path, payload)
				return
			}
			printPost(path, payload)
		},
	}
	cmd.Flags().StringVar(&o.AccountID, "account-id", "", "account id (required)")
	cmd.Flags().StringVar(&o.Date, "date", "", "date YYYY-MM-DD (required)")
	cmd.Flags().StringVar(&o.Type, "type", "", "trade type: buy|sell (required)")
	cmd.Flags().StringVar(&o.Qty, "qty", "", "quantity (required)")
	cmd.Flags().StringVar(&o.Price, "price", "", "price (required)")
	cmd.Flags().StringVar(&o.Currency, "currency", "", "currency")
	cmd.Flags().StringVar(&o.SecurityID, "security-id", "", "security id")
	cmd.Flags().StringVar(&o.Ticker, "ticker", "", "ticker")
	cmd.Flags().StringVar(&o.ManualTicker, "manual-ticker", "", "manual ticker")
	cmd.Flags().StringVar(&o.InvestmentActivityLabel, "investment-activity-label", "", "investment activity label")
	cmd.Flags().StringVar(&o.CategoryID, "category-id", "", "category id")
	cmd.Flags().BoolVar(&o.Apply, "apply", false, "execute the create (otherwise dry-run)")
	return cmd
}

func newTradesUpdateCmd() *cobra.Command {
	var o tradeUpdateOpts
	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update trade (default dry-run; use --apply to execute)",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			payload, err := buildTradeUpdatePayload(o)
			if err != nil {
				output.Fail("validation_failed", err.Error(), nil)
			}
			path := fmt.Sprintf("/api/v1/trades/%s", args[0])
			if !o.Apply {
				printDryRun("PATCH", path, payload)
				return
			}
			printPatch(path, payload)
		},
	}
	cmd.Flags().StringVar(&o.Name, "name", "", "name")
	cmd.Flags().StringVar(&o.Date, "date", "", "date YYYY-MM-DD")
	cmd.Flags().StringVar(&o.Amount, "amount", "", "amount")
	cmd.Flags().StringVar(&o.Currency, "currency", "", "currency")
	cmd.Flags().StringVar(&o.Notes, "notes", "", "notes")
	cmd.Flags().StringVar(&o.Nature, "nature", "", "nature: inflow|outflow")
	cmd.Flags().StringVar(&o.Type, "type", "", "trade type: buy|sell")
	cmd.Flags().StringVar(&o.Qty, "qty", "", "quantity")
	cmd.Flags().StringVar(&o.Price, "price", "", "price")
	cmd.Flags().StringVar(&o.InvestmentActivityLabel, "investment-activity-label", "", "investment activity label")
	cmd.Flags().StringVar(&o.CategoryID, "category-id", "", "category id")
	cmd.Flags().BoolVar(&o.Apply, "apply", false, "execute the update (otherwise dry-run)")
	return cmd
}

func newTradesDeleteCmd() *cobra.Command {
	var apply bool
	cmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete trade (default dry-run; use --apply to execute)",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			path := fmt.Sprintf("/api/v1/trades/%s", args[0])
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

func buildTradeCreatePayload(o tradeCreateOpts) (map[string]any, error) {
	if o.AccountID == "" {
		return nil, fmt.Errorf("account-id is required")
	}
	if o.Date == "" {
		return nil, fmt.Errorf("date is required")
	}
	if o.Type != "buy" && o.Type != "sell" {
		return nil, fmt.Errorf("type must be buy or sell")
	}
	if o.Qty == "" || o.Price == "" {
		return nil, fmt.Errorf("qty and price are required")
	}
	if o.SecurityID == "" && o.Ticker == "" && o.ManualTicker == "" {
		return nil, fmt.Errorf("one of security-id, ticker, or manual-ticker is required")
	}
	trade := map[string]any{
		"account_id": o.AccountID,
		"date":       o.Date,
		"type":       o.Type,
		"qty":        o.Qty,
		"price":      o.Price,
	}
	addAny(trade, "currency", o.Currency)
	addAny(trade, "security_id", o.SecurityID)
	addAny(trade, "ticker", o.Ticker)
	addAny(trade, "manual_ticker", o.ManualTicker)
	addAny(trade, "investment_activity_label", o.InvestmentActivityLabel)
	addAny(trade, "category_id", o.CategoryID)
	return map[string]any{"trade": trade}, nil
}

func buildTradeUpdatePayload(o tradeUpdateOpts) (map[string]any, error) {
	trade := map[string]any{}
	addAny(trade, "name", o.Name)
	addAny(trade, "date", o.Date)
	addAny(trade, "amount", o.Amount)
	addAny(trade, "currency", o.Currency)
	addAny(trade, "notes", o.Notes)
	addAny(trade, "nature", o.Nature)
	addAny(trade, "type", o.Type)
	addAny(trade, "qty", o.Qty)
	addAny(trade, "price", o.Price)
	addAny(trade, "investment_activity_label", o.InvestmentActivityLabel)
	addAny(trade, "category_id", o.CategoryID)
	if len(trade) == 0 {
		return nil, fmt.Errorf("no fields provided to update")
	}
	return map[string]any{"trade": trade}, nil
}

func addAny(m map[string]any, key, value string) {
	if value != "" {
		m[key] = value
	}
}
