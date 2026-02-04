package root

import (
	"fmt"

	"github.com/dgilperez/sure-cli/internal/api"
	"github.com/dgilperez/sure-cli/internal/output"
	"github.com/spf13/cobra"
)

func newHoldingsCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "holdings", Short: "Investment holdings (requires Sure API support)"}
	cmd.AddCommand(newHoldingsListCmd())
	cmd.AddCommand(newHoldingsPerformanceCmd())
	return cmd
}

func newHoldingsListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List investment holdings",
		Run: func(cmd *cobra.Command, args []string) {
			client := api.New()

			// Try the holdings endpoint (may not exist in all Sure versions)
			var res any
			r, err := client.Get("/api/v1/holdings", &res)
			if err != nil {
				output.Fail("request_failed", err.Error(), nil)
			}

			if r.StatusCode() == 404 {
				output.Fail("not_implemented", "Holdings API not available. This requires Sure with investment account support.", map[string]any{
					"endpoint": "/api/v1/holdings",
					"hint":     "Ensure your Sure instance supports investment accounts",
				})
			}

			_ = output.Print(format, output.Envelope{Data: res, Meta: &output.Meta{Status: r.StatusCode()}})
		},
	}
	return cmd
}

func newHoldingsPerformanceCmd() *cobra.Command {
	var period string

	cmd := &cobra.Command{
		Use:   "performance",
		Short: "Investment performance summary",
		Run: func(cmd *cobra.Command, args []string) {
			client := api.New()

			// Try the performance endpoint
			path := "/api/v1/holdings/performance"
			if period != "" {
				path = fmt.Sprintf("%s?period=%s", path, period)
			}

			var res any
			r, err := client.Get(path, &res)
			if err != nil {
				output.Fail("request_failed", err.Error(), nil)
			}

			if r.StatusCode() == 404 {
				output.Fail("not_implemented", "Holdings performance API not available.", map[string]any{
					"endpoint": path,
					"hint":     "This feature requires Sure with investment tracking enabled",
				})
			}

			_ = output.Print(format, output.Envelope{Data: res, Meta: &output.Meta{Status: r.StatusCode()}})
		},
	}
	cmd.Flags().StringVar(&period, "period", "1m", "performance period (1w|1m|3m|6m|1y|ytd|all)")
	return cmd
}
