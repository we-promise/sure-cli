package root

import (
	"fmt"
	"time"

	"github.com/we-promise/sure-cli/internal/api"
	"github.com/we-promise/sure-cli/internal/output"
	"github.com/we-promise/sure-cli/internal/plan"
	"github.com/spf13/cobra"
)

func newPlanCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "plan", Short: "Planning commands (budget/runway/forecast)"}
	cmd.AddCommand(newPlanBudgetCmd())
	cmd.AddCommand(newPlanRunwayCmd())
	cmd.AddCommand(newPlanForecastCmd())
	return cmd
}

func newPlanForecastCmd() *cobra.Command {
	var days int
	var includeDaily bool
	var months int

	cmd := &cobra.Command{
		Use:   "forecast",
		Short: "Forecast spending for the next N days",
		Run: func(cmd *cobra.Command, args []string) {
			client := api.New()

			// Fetch historical data (use months for lookback)
			if months <= 0 {
				months = 6
			}
			end := time.Now().UTC()
			start := end.AddDate(0, -months, 0)
			txs, err := api.FetchTransactionsWindow(client, start, end, 500)
			if err != nil {
				output.Fail("request_failed", err.Error(), nil)
			}

			result := plan.ComputeForecast(txs, days, includeDaily)
			_ = output.Print(format, output.Envelope{Data: result, Meta: &output.Meta{Schema: "docs/schemas/v1/plan_forecast.schema.json", Status: 200}})
		},
	}
	cmd.Flags().IntVar(&days, "days", 30, "forecast period in days")
	cmd.Flags().IntVar(&months, "months", 6, "historical lookback months")
	cmd.Flags().BoolVar(&includeDaily, "daily", false, "include daily breakdown")
	return cmd
}

func newPlanBudgetCmd() *cobra.Command {
	var monthStr string
	cmd := &cobra.Command{
		Use:   "budget",
		Short: "Budget pacing for a month (client-side heuristic)",
		Run: func(cmd *cobra.Command, args []string) {
			m := time.Now().UTC()
			if monthStr != "" {
				mm, err := time.Parse("2006-01", monthStr)
				if err != nil {
					output.Fail("invalid_month", "month must be YYYY-MM", nil)
				}
				m = mm
			}

			client := api.New()
			start := time.Date(m.Year(), m.Month(), 1, 0, 0, 0, 0, time.UTC)
			end := start.AddDate(0, 1, 0)
			txs, err := api.FetchTransactionsWindow(client, start, end, 200)
			if err != nil {
				output.Fail("request_failed", err.Error(), nil)
			}
			res, err := plan.ComputeMonthlyBudget(m, txs)
			if err != nil {
				output.Fail("compute_failed", err.Error(), nil)
			}
			_ = output.Print(format, output.Envelope{Data: res, Meta: &output.Meta{Schema: "docs/schemas/v1/plan_budget.schema.json", Status: 200}})
		},
	}
	cmd.Flags().StringVar(&monthStr, "month", "", "month YYYY-MM")
	return cmd
}

func newPlanRunwayCmd() *cobra.Command {
	var accountID string
	var windowDays int
	cmd := &cobra.Command{
		Use:   "runway",
		Short: "Estimate runway months based on recent spending",
		Run: func(cmd *cobra.Command, args []string) {
			if accountID == "" {
				output.Fail("missing_account", "--account-id is required", nil)
			}
			client := api.New()

			// Find account balance by listing accounts (Sure API quirks: show may 404)
			var res map[string]any
			_, err := client.Get("/api/v1/accounts", &res)
			if err != nil {
				output.Fail("request_failed", err.Error(), nil)
			}
			bal := ""
			if arr, ok := res["accounts"].([]any); ok {
				for _, it := range arr {
					m, _ := it.(map[string]any)
					if fmt.Sprint(m["id"]) == accountID {
						bal = fmt.Sprint(m["balance"])
						break
					}
				}
			}
			if bal == "" {
				output.Fail("account_not_found", "account not found in accounts list", map[string]any{"account_id": accountID})
			}

			if windowDays <= 0 {
				windowDays = 90
			}

			end := time.Now().UTC()
			start := end.AddDate(0, 0, -windowDays)
			txs, err := api.FetchTransactionsWindow(client, start, end, 200)
			if err != nil {
				output.Fail("request_failed", err.Error(), nil)
			}

			out, err := plan.ComputeRunway(bal, txs, windowDays)
			if err != nil {
				output.Fail("compute_failed", err.Error(), nil)
			}
			_ = output.Print(format, output.Envelope{Data: out, Meta: &output.Meta{Schema: "docs/schemas/v1/plan_runway.schema.json", Status: 200}})
		},
	}
	cmd.Flags().StringVar(&accountID, "account-id", "", "cash account id")
	cmd.Flags().IntVar(&windowDays, "days", 90, "lookback days")
	return cmd
}
