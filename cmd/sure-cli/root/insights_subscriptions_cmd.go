package root

import (
	"time"

	"github.com/dgilperez/sure-cli/internal/api"
	"github.com/dgilperez/sure-cli/internal/insights"
	"github.com/dgilperez/sure-cli/internal/output"
	"github.com/spf13/cobra"
)

func newInsightsSubscriptionsCmd() *cobra.Command {
	var months int

	cmd := &cobra.Command{
		Use:   "subscriptions",
		Short: "Detect likely subscriptions (recurring expenses)",
		Run: func(cmd *cobra.Command, args []string) {
			end := time.Now()
			start := end.AddDate(0, -months, 0)

			txs, err := api.FetchTransactionsWindow(api.New(), start, end, 100)
			if err != nil {
				output.Fail("request_failed", err.Error(), nil)
			}
			cands := insights.DetectSubscriptions(txs)
			if cands == nil {
				cands = []insights.SubscriptionCandidate{}
			}
			_ = output.Print(format, output.Envelope{Data: map[string]any{
				"window":     map[string]any{"start": start.Format("2006-01-02"), "end": end.Format("2006-01-02")},
				"candidates": cands,
			}, Meta: &output.Meta{Schema: "docs/schemas/v1/insights_subscriptions.schema.json"}})
		},
	}
	cmd.Flags().IntVar(&months, "months", 6, "lookback months")
	return cmd
}
