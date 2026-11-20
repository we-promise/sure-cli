package root

import (
	"errors"
	"fmt"
	"math"
	"net/url"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/we-promise/sure-cli/internal/analyze"
	"github.com/we-promise/sure-cli/internal/api"
	"github.com/we-promise/sure-cli/internal/money"
	"github.com/we-promise/sure-cli/internal/output"
)

// jsonOnly tells --format table users that a command has no table renderer.
const jsonOnly = " Output is JSON only; --format table prints the same JSON."

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
	var months, minCount int
	var startDate, endDate, currency string
	var minTotal, maxAvg float64

	cmd := &cobra.Command{
		Use:   "leaks",
		Short: "Detect spending leaks (small frequent expenses that add up)",
		Long: "Groups the window's expenses by merchant (falling back to the transaction name), " +
			"one group per currency, and keeps the small, frequent ones. By default a group " +
			"needs a total of at least 1500 and an average of at most 1000 minor units of its " +
			"currency (15.00 and 10.00 EUR, ¥1500 and ¥1000). --min-total and --max-avg are " +
			"major units of --currency, which they require. The output echoes the window and " +
			"the thresholds applied, in minor units." + jsonOnly,
		Args: cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			start, end, err := leaksWindow(months, startDate, endDate, time.Now())
			failValidation(err)
			params, err := leakParams(cmd.Flags().Changed, minCount, minTotal, maxAvg, currency)
			failValidation(err)
			txs, err := api.FetchTransactionsWindow(api.New(), start, end, api.MaxPerPage)
			checkResponse(nil, err)
			cands := analyze.DetectLeaks(txs, start, end, params)
			if cands == nil {
				cands = []analyze.LeakCandidate{}
			}
			if err := output.Print(format, output.Envelope{Data: map[string]any{
				"window": map[string]any{
					"start": start.Format(time.DateOnly),
					"end":   end.Format(time.DateOnly),
					"days":  analyze.InclusiveDays(start, end),
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
	cmd.Flags().StringVar(&currency, "currency", "", "only expenses in this currency (default: every currency, grouped apart)")
	cmd.Flags().IntVar(&minCount, "min-count", analyze.DefaultLeakParams.MinCount, "minimum occurrences")
	cmd.Flags().Float64Var(&minTotal, "min-total", 0, "minimum total spend, major units of --currency (default 1500 minor units of each group's currency)")
	cmd.Flags().Float64Var(&maxAvg, "max-avg", 0, "maximum average per transaction, major units of --currency (default 1000 minor units of each group's currency)")
	return cmd
}

// leakParams resolves the leaks flags into the thresholds DetectLeaks
// applies. --min-total and --max-avg replace the defaults only when given,
// so 0 is a real value, and they need --currency to know their minor unit.
func leakParams(changed func(flag string) bool, minCount int, minTotal, maxAvg float64, currency string) (analyze.LeakParams, error) {
	p := analyze.DefaultLeakParams
	p.MinCount = minCount
	p.Currency = strings.ToUpper(strings.TrimSpace(currency))
	for _, t := range []struct {
		flag  string
		value float64
		minor *float64
	}{{"min-total", minTotal, &p.MinTotalMinor}, {"max-avg", maxAvg, &p.MaxAvgMinor}} {
		if !changed(t.flag) {
			continue
		}
		if p.Currency == "" {
			return p, fmt.Errorf("--%s is in major units of one currency: pass --currency too", t.flag)
		}
		*t.minor = math.Round(t.value * money.MinorUnitConversion(p.Currency))
	}
	return p, p.Validate()
}

// today is now's calendar date in UTC, the day Sure's date filters use.
func today(now time.Time) time.Time {
	now = now.UTC()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
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
	end := today(now)
	return lookbackStart(end, months), end, nil
}

// runwayWindow is the last `days` calendar days up to today, inclusive.
func runwayWindow(days int, now time.Time) (time.Time, time.Time, error) {
	if days <= 0 {
		return time.Time{}, time.Time{}, errors.New("days must be positive")
	}
	end := today(now)
	return end.AddDate(0, 0, 1-days), end, nil
}

func newAnalyzeRunwayCmd() *cobra.Command {
	var accountID string
	var windowDays int
	cmd := &cobra.Command{
		Use:   "runway",
		Short: "Estimate runway months based on recent spending",
		Long: "Divides the account's balance by the monthly burn: the whole family's reportable " +
			"expenses in the account currency over the last --days days, from every account, " +
			"extrapolated to 30-day months.",
		Args: cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			if accountID == "" {
				output.Fail("missing_account", "--account-id is required", nil)
				return
			}
			start, end, err := runwayWindow(windowDays, time.Now())
			failValidation(err)
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

			txs, err := api.FetchTransactionsWindow(client, start, end, api.MaxPerPage)
			checkResponse(nil, err)

			out := analyze.ComputeRunway(bal, fmt.Sprint(acc["currency"]), txs, start, end)
			if err := output.Print(format, output.Envelope{Data: out, Meta: &output.Meta{Schema: "docs/schemas/v1/analyze_runway.schema.json", Status: 200}}); err != nil {
				output.Fail("output_failed", err.Error(), nil)
			}
		},
	}
	cmd.Flags().StringVar(&accountID, "account-id", "", "cash account id")
	cmd.Flags().IntVar(&windowDays, "days", 90, "lookback days, today included")
	return cmd
}
