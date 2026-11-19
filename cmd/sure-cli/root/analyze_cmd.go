package root

import (
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/spf13/cobra"

	"github.com/we-promise/sure-cli/internal/analyze"
	"github.com/we-promise/sure-cli/internal/api"
	"github.com/we-promise/sure-cli/internal/output"
)

func newAnalyzeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "analyze",
		Short: "Client-side heuristics Sure does not compute (leaks, runway)",
	}
	cmd.AddCommand(newAnalyzeLeaksCmd())
	cmd.AddCommand(newAnalyzeRunwayCmd())
	return cmd
}

func newAnalyzeLeaksCmd() *cobra.Command {
	var months int
	var startDate, endDate string
	params := analyze.DefaultLeakParams

	cmd := &cobra.Command{
		Use:   "leaks",
		Short: "Detect spending leaks (small frequent expenses that add up)",
		Long: "Groups the window's expenses by merchant (falling back to the transaction name), " +
			"one group per currency, and keeps the small, frequent ones. Amount thresholds " +
			"are in major units of each group's currency.",
		Args: cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			start, end, err := leaksWindow(months, startDate, endDate, time.Now())
			failValidation(err)
			txs, err := api.FetchTransactionsWindow(api.New(), start, end, api.MaxPerPage)
			if err != nil {
				output.Fail("request_failed", err.Error(), nil)
			}
			cands := analyze.DetectLeaks(txs, start, end, params)
			if cands == nil {
				cands = []analyze.LeakCandidate{}
			}
			if err := output.Print(format, output.Envelope{Data: map[string]any{
				"window": map[string]any{
					"start": start.Format(isoDateLayout),
					"end":   end.Format(isoDateLayout),
					"days":  int(end.Sub(start).Hours()/24) + 1,
				},
				"params":     params,
				"candidates": cands,
			}, Meta: &output.Meta{Schema: "docs/schemas/v1/analyze_leaks.schema.json"}}); err != nil {
				output.Fail("output_failed", err.Error(), nil)
			}
		},
	}
	cmd.Flags().IntVar(&months, "months", 3, "lookback months ending today (ignored with --start-date/--end-date)")
	cmd.Flags().StringVar(&startDate, "start-date", "", "window start YYYY-MM-DD (with --end-date)")
	cmd.Flags().StringVar(&endDate, "end-date", "", "window end YYYY-MM-DD, inclusive (with --start-date)")
	cmd.Flags().StringVar(&params.Currency, "currency", "", "only expenses in this currency (default: every currency, grouped apart)")
	cmd.Flags().IntVar(&params.MinCount, "min-count", params.MinCount, "minimum occurrences")
	cmd.Flags().Float64Var(&params.MinTotal, "min-total", params.MinTotal, "minimum total spend")
	cmd.Flags().Float64Var(&params.MaxAvg, "max-avg", params.MaxAvg, "maximum average per transaction")
	return cmd
}

// leaksWindow resolves the analysis window in whole days: --start-date and
// --end-date together, else the last `months` months up to today.
func leaksWindow(months int, startDate, endDate string, now time.Time) (time.Time, time.Time, error) {
	explicit, err := parsePairedDateRange(startDate, endDate)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	if explicit {
		return parseOptionalDateRange(startDate, endDate)
	}
	if months <= 0 {
		return time.Time{}, time.Time{}, errors.New("months must be positive")
	}
	end := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	return end.AddDate(0, -months, 0), end, nil
}

func newAnalyzeRunwayCmd() *cobra.Command {
	var accountID string
	var windowDays int
	cmd := &cobra.Command{
		Use:   "runway",
		Short: "Estimate runway months based on recent spending",
		Run: func(cmd *cobra.Command, args []string) {
			if accountID == "" {
				output.Fail("missing_account", "--account-id is required", nil)
				return
			}
			client := api.New()

			// Fetch the account directly; include_disabled so a disabled
			// account can still be planned against when asked for by id.
			var acc map[string]any
			path := pathWithQuery("/api/v1/accounts/"+url.PathEscape(accountID), api.AccountsQuery(true))
			r, err := client.Get(path, &acc)
			checkResponse(r, err)
			bal, ok := amountFromAPIOK(acc, "balance", "balance_cents")
			if !ok {
				output.Fail("account_balance_missing", "account has no usable balance", map[string]any{"account_id": accountID})
				return
			}

			if windowDays <= 0 {
				windowDays = 90
			}

			end := time.Now().UTC()
			start := end.AddDate(0, 0, -windowDays)
			txs, err := api.FetchTransactionsWindow(client, start, end, 200)
			if err != nil {
				output.Fail("request_failed", err.Error(), nil)
				return
			}

			out := analyze.ComputeRunway(bal, fmt.Sprint(acc["currency"]), txs, windowDays)
			_ = output.Print(format, output.Envelope{Data: out, Meta: &output.Meta{Schema: "docs/schemas/v1/analyze_runway.schema.json", Status: 200}})
		},
	}
	cmd.Flags().StringVar(&accountID, "account-id", "", "cash account id")
	cmd.Flags().IntVar(&windowDays, "days", 90, "lookback days")
	return cmd
}
