package root

import "github.com/spf13/cobra"

func newInsightsCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "insights", Short: "JTBD-oriented insights (Phase 4)"}
	cmd.AddCommand(newInsightsSubscriptionsCmd())
	cmd.AddCommand(newInsightsFeesCmd())
	cmd.AddCommand(newInsightsLeaksCmd())
	return cmd
}
