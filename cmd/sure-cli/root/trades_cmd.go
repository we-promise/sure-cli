package root

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/spf13/cobra"
	"github.com/we-promise/sure-cli/internal/models"
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
			printGet(u.String())
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
			path := fmt.Sprintf("/api/v1/trades/%s", url.PathEscape(args[0]))
			printGet(path)
		},
	})

	cmd.AddCommand(newTradesCreateCmd())
	cmd.AddCommand(newTradesUpdateCmd())
	cmd.AddCommand(newTradesDeleteCmd())

	return cmd
}

// securityTradeLabels mirrors Trade::CreateForm::SECURITY_TRADE_LABELS: the
// types that move shares, and the only label Sure accepts for each. Order is
// the one shown in help and error messages.
var securityTradeLabels = []struct{ Type, Label string }{
	{"buy", "Buy"},
	{"sell", "Sell"},
	{"sweep_in", "Sweep In"},
	{"sweep_out", "Sweep Out"},
	{"reinvestment", "Reinvestment"},
}

func securityTradeLabel(typ string) (string, bool) {
	for _, l := range securityTradeLabels {
		if l.Type == typ {
			return l.Label, true
		}
	}
	return "", false
}

type securityUse int

const (
	securityForbidden securityUse = iota
	securityOptional
	securityRequired
)

// tradeCreateRule is what the branch of Sure's TradesController#create for a
// type reads: share types need qty and price (fee optional), every other type
// needs a positive amount. Flags a branch never reads are rejected instead of
// being silently dropped by Sure.
type tradeCreateRule struct {
	Security securityUse
	Shares   bool
	Transfer bool
}

type tradeCreateType struct {
	Type string
	Rule tradeCreateRule
}

// tradeCreateTypes mirrors Trade::CreateForm::SUPPORTED_TYPES.
var tradeCreateTypes = func() []tradeCreateType {
	var types []tradeCreateType
	for _, l := range securityTradeLabels {
		types = append(types, tradeCreateType{l.Type, tradeCreateRule{Security: securityRequired, Shares: true}})
	}
	return append(types,
		tradeCreateType{"dividend", tradeCreateRule{Security: securityRequired}},
		tradeCreateType{"interest", tradeCreateRule{Security: securityOptional}},
		tradeCreateType{"fee", tradeCreateRule{Security: securityOptional}},
		tradeCreateType{"deposit", tradeCreateRule{Transfer: true}},
		tradeCreateType{"withdrawal", tradeCreateRule{Transfer: true}},
	)
}()

func tradeCreateRuleFor(typ string) (tradeCreateRule, bool) {
	for _, t := range tradeCreateTypes {
		if t.Type == typ {
			return t.Rule, true
		}
	}
	return tradeCreateRule{}, false
}

func tradeCreateTypeNames() string {
	names := make([]string, len(tradeCreateTypes))
	for i, t := range tradeCreateTypes {
		names[i] = t.Type
	}
	return strings.Join(names, ", ")
}

func securityTradeTypeNames() string {
	names := make([]string, len(securityTradeLabels))
	for i, l := range securityTradeLabels {
		names[i] = l.Type
	}
	return strings.Join(names, ", ")
}

// normalizeTradeType matches Sure, which downcases the type param.
func normalizeTradeType(typ string) string {
	return strings.ToLower(strings.TrimSpace(typ))
}

type tradeCreateOpts struct {
	AccountID         string
	Date              string
	Type              string
	Qty               string
	Price             string
	Fee               string
	Amount            string
	Currency          string
	SecurityID        string
	Ticker            string
	ManualTicker      string
	TransferAccountID string
	Apply             bool
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
	Apply                   bool
}

func newTradesCreateCmd() *cobra.Command {
	var o tradeCreateOpts
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create trade or investment activity (default dry-run; use --apply to execute)",
		Long: `Create a trade or investment-account activity.

Required flags depend on --type:
  buy, sell, sweep_in, sweep_out, reinvestment: --qty, --price and one security (--fee optional)
  dividend:            --amount and one security
  interest, fee:       --amount (security optional)
  deposit, withdrawal: --amount (--transfer-account-id optional)

Sure stores the activity label from the type. The response shape depends on the
type: share types, dividend, interest and fee return a trade; deposit and
withdrawal return a transaction, or a transfer when --transfer-account-id is set.`,
		Run: func(cmd *cobra.Command, args []string) {
			payload, err := buildTradeCreatePayload(o)
			failValidation(err)
			dispatchWrite(o.Apply, "POST", "/api/v1/trades", payload)
		},
	}
	cmd.Flags().StringVar(&o.AccountID, "account-id", "", "investment or crypto exchange account id (required)")
	cmd.Flags().StringVar(&o.Date, "date", "", "date YYYY-MM-DD (required)")
	cmd.Flags().StringVar(&o.Type, "type", "", "type (required): "+tradeCreateTypeNames())
	cmd.Flags().StringVar(&o.Qty, "qty", "", "quantity, positive (share types)")
	cmd.Flags().StringVar(&o.Price, "price", "", "price per share, positive (share types)")
	cmd.Flags().StringVar(&o.Fee, "fee", "", "trade fee, 0 or more (share types)")
	cmd.Flags().StringVar(&o.Amount, "amount", "", "amount, positive (dividend, interest, fee, deposit, withdrawal)")
	cmd.Flags().StringVar(&o.Currency, "currency", "", "currency (defaults to the account currency)")
	cmd.Flags().StringVar(&o.SecurityID, "security-id", "", "security id")
	cmd.Flags().StringVar(&o.Ticker, "ticker", "", "ticker, optionally TICKER|MIC")
	cmd.Flags().StringVar(&o.ManualTicker, "manual-ticker", "", "manual (offline) ticker")
	cmd.Flags().StringVar(&o.TransferAccountID, "transfer-account-id", "", "other account of a deposit or withdrawal")
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
			failValidation(err)
			dispatchWrite(o.Apply, "PATCH", fmt.Sprintf("/api/v1/trades/%s", url.PathEscape(args[0])), payload)
		},
	}
	cmd.Flags().StringVar(&o.Name, "name", "", "name")
	cmd.Flags().StringVar(&o.Date, "date", "", "date YYYY-MM-DD")
	cmd.Flags().StringVar(&o.Amount, "amount", "", "entry amount, signed as Sure stores it (positive = outflow); not with --qty/--price, from which Sure recomputes it")
	cmd.Flags().StringVar(&o.Currency, "currency", "", "currency")
	cmd.Flags().StringVar(&o.Notes, "notes", "", "notes")
	cmd.Flags().StringVar(&o.Nature, "nature", "", "inflow (sell) or outflow (buy); needs --qty or --price, not with --type")
	cmd.Flags().StringVar(&o.Type, "type", "", "trade type, needs --qty or --price: "+securityTradeTypeNames())
	cmd.Flags().StringVar(&o.Qty, "qty", "", "quantity, positive")
	cmd.Flags().StringVar(&o.Price, "price", "", "price per share, positive")
	cmd.Flags().StringVar(&o.InvestmentActivityLabel, "investment-activity-label", "", "investment activity label (with --type it must be that type's label, e.g. sweep_in -> \"Sweep In\")")
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
			dispatchWrite(apply, "DELETE", fmt.Sprintf("/api/v1/trades/%s", url.PathEscape(args[0])), nil)
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
	if _, err := parseISODate("date", o.Date); err != nil {
		return nil, err
	}
	typ := normalizeTradeType(o.Type)
	rule, ok := tradeCreateRuleFor(typ)
	if !ok {
		return nil, fmt.Errorf("type must be one of %s", tradeCreateTypeNames())
	}
	if err := checkTradeCreateFields(o, typ, rule); err != nil {
		return nil, err
	}

	trade := map[string]any{
		"account_id": o.AccountID,
		"date":       o.Date,
		"type":       typ,
	}
	addAny(trade, "qty", o.Qty)
	addAny(trade, "price", o.Price)
	addAny(trade, "fee", o.Fee)
	addAny(trade, "amount", o.Amount)
	addAny(trade, "currency", o.Currency)
	addAny(trade, "security_id", o.SecurityID)
	addAny(trade, "ticker", o.Ticker)
	addAny(trade, "manual_ticker", o.ManualTicker)
	addAny(trade, "transfer_account_id", o.TransferAccountID)
	return map[string]any{"trade": trade}, nil
}

type tradeFlag struct{ Name, Value string }

func checkTradeCreateFields(o tradeCreateOpts, typ string, rule tradeCreateRule) error {
	notApplicable := func(flags ...tradeFlag) error {
		for _, f := range flags {
			if f.Value != "" {
				return fmt.Errorf("%s does not apply to %s", f.Name, typ)
			}
		}
		return nil
	}

	securityFlags := []tradeFlag{{"security-id", o.SecurityID}, {"ticker", o.Ticker}, {"manual-ticker", o.ManualTicker}}
	given := 0
	for _, f := range securityFlags {
		if f.Value != "" {
			given++
		}
	}
	switch {
	case rule.Security == securityForbidden:
		if err := notApplicable(securityFlags...); err != nil {
			return err
		}
	case given > 1:
		return fmt.Errorf("provide only one of security-id, ticker, or manual-ticker")
	case given == 0 && rule.Security == securityRequired:
		return fmt.Errorf("one of security-id, ticker, or manual-ticker is required for %s", typ)
	}

	if rule.Shares {
		if err := notApplicable(tradeFlag{"amount", o.Amount}); err != nil {
			return err
		}
		if err := requirePositiveDecimal("qty", o.Qty, tradeQtyColumn); err != nil {
			return err
		}
		if err := requirePositiveDecimal("price", o.Price, tradePriceColumn); err != nil {
			return err
		}
		if o.Fee != "" {
			// Sure adds the fee to the trade's cost; a negative one would
			// silently lower it.
			if !models.IsDecimal(o.Fee) || strings.HasPrefix(o.Fee, "-") {
				return fmt.Errorf("fee must be a decimal number of 0 or more, got %q", o.Fee)
			}
			if err := moneyColumn.check("fee", o.Fee); err != nil {
				return err
			}
		}
	} else {
		if err := notApplicable(tradeFlag{"qty", o.Qty}, tradeFlag{"price", o.Price}, tradeFlag{"fee", o.Fee}); err != nil {
			return err
		}
		if err := requirePositiveDecimal("amount", o.Amount, moneyColumn); err != nil {
			return err
		}
	}

	if !rule.Transfer {
		return notApplicable(tradeFlag{"transfer-account-id", o.TransferAccountID})
	}
	// Transfer::Creator books each side in its own account's currency.
	if o.TransferAccountID != "" && o.Currency != "" {
		return fmt.Errorf("currency does not apply to a %s with transfer-account-id; each account keeps its own currency", typ)
	}
	return nil
}

func buildTradeUpdatePayload(o tradeUpdateOpts) (map[string]any, error) {
	typ := normalizeTradeType(o.Type)
	if typ != "" {
		label, ok := securityTradeLabel(typ)
		if !ok {
			return nil, fmt.Errorf("type must be one of %s", securityTradeTypeNames())
		}
		if o.InvestmentActivityLabel != "" && o.InvestmentActivityLabel != label {
			return nil, fmt.Errorf("investment-activity-label must be %q for type %s", label, typ)
		}
	}

	if err := checkTradeUpdateFields(o, typ); err != nil {
		return nil, err
	}

	trade := map[string]any{}
	addAny(trade, "name", o.Name)
	addAny(trade, "date", o.Date)
	addAny(trade, "amount", o.Amount)
	addAny(trade, "currency", o.Currency)
	addAny(trade, "notes", o.Notes)
	addAny(trade, "nature", o.Nature)
	addAny(trade, "type", typ)
	addAny(trade, "qty", o.Qty)
	addAny(trade, "price", o.Price)
	addAny(trade, "investment_activity_label", o.InvestmentActivityLabel)
	if len(trade) == 0 {
		return nil, fmt.Errorf("no fields provided to update")
	}
	return map[string]any{"trade": trade}, nil
}

// checkTradeUpdateFields rejects what Sure's TradesController#update would
// ignore or overwrite while answering 200: it reads type or nature only to
// sign a qty/price change (type first), recomputes the amount as
// qty × price when either is given, and casts qty and price with to_d, so
// "abc" would be saved as 0.
func checkTradeUpdateFields(o tradeUpdateOpts, typ string) error {
	shares := o.Qty != "" || o.Price != ""
	switch {
	case typ != "" && o.Nature != "":
		return fmt.Errorf("use either type or nature, not both")
	case o.Nature != "" && o.Nature != "inflow" && o.Nature != "outflow":
		return fmt.Errorf("nature must be inflow or outflow, got %q", o.Nature)
	case (typ != "" || o.Nature != "") && !shares:
		return fmt.Errorf("type and nature only apply with qty or price; Sure ignores them otherwise")
	case o.Amount != "" && shares:
		return fmt.Errorf("amount cannot be combined with qty or price; Sure recomputes it as qty × price")
	}
	if o.Qty != "" {
		if err := requirePositiveDecimal("qty", o.Qty, tradeQtyColumn); err != nil {
			return err
		}
	}
	if o.Price != "" {
		if err := requirePositiveDecimal("price", o.Price, tradePriceColumn); err != nil {
			return err
		}
	}
	if _, err := parseOptionalISODate("date", o.Date); err != nil {
		return err
	}
	return checkOptionalDecimal("amount", o.Amount, moneyColumn)
}

func addAny(m map[string]any, key, value string) {
	if value != "" {
		m[key] = value
	}
}
