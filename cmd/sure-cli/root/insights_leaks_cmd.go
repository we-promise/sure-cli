package root

import (
	"time"

	"github.com/dgilperez/sure-cli/internal/insights"
	"github.com/dgilperez/sure-cli/internal/output"
	"github.com/spf13/cobra"
)

func newInsightsLeaksCmd() *cobra.Command {
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
			txs, err := fetchTransactionsWindow(start, end)
			if err != nil {
				output.Fail("request_failed", err.Error(), nil)
			}
			cands := insights.DetectLeaks(txs, minCount, minTotal, maxAvg)
			_ = output.Print(format, output.Envelope{Data: map[string]any{
				"window":     map[string]any{"start": start.Format("2006-01-02"), "end": end.Format("2006-01-02")},
				"params":     map[string]any{"min_count": minCount, "min_total": minTotal, "max_avg": maxAvg},
				"candidates": cands,
			}})
		},
	}
	cmd.Flags().IntVar(&months, "months", 3, "lookback months")
	cmd.Flags().IntVar(&minCount, "min-count", 3, "minimum occurrences")
	cmd.Flags().Float64Var(&minTotal, "min-total", 15, "minimum total spend")
	cmd.Flags().Float64Var(&maxAvg, "max-avg", 10, "maximum average per transaction")
	return cmd
}
