package root

import (
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
	var minCount int
	var minTotal float64
	var maxAvg float64

	cmd := &cobra.Command{
		Use:   "leaks",
		Short: "Detect spending leaks (small frequent expenses that add up)",
		Run: func(cmd *cobra.Command, args []string) {
			end := time.Now()
			start := end.AddDate(0, -months, 0)
			txs, err := api.FetchTransactionsWindow(api.New(), start, end, 100)
			if err != nil {
				output.Fail("request_failed", err.Error(), nil)
				return
			}
			cands := analyze.DetectLeaks(txs, minCount, minTotal, maxAvg)
			if cands == nil {
				cands = []analyze.LeakCandidate{}
			}
			_ = output.Print(format, output.Envelope{Data: map[string]any{
				"window":     map[string]any{"start": start.Format("2006-01-02"), "end": end.Format("2006-01-02")},
				"params":     map[string]any{"min_count": minCount, "min_total": minTotal, "max_avg": maxAvg},
				"candidates": cands,
			}, Meta: &output.Meta{Schema: "docs/schemas/v1/analyze_leaks.schema.json"}})
		},
	}
	cmd.Flags().IntVar(&months, "months", 3, "lookback months")
	cmd.Flags().IntVar(&minCount, "min-count", 3, "minimum occurrences")
	cmd.Flags().Float64Var(&minTotal, "min-total", 15, "minimum total spend")
	cmd.Flags().Float64Var(&maxAvg, "max-avg", 10, "maximum average per transaction")
	return cmd
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
