package root

import (
	"fmt"
	"time"

	"github.com/dgilperez/sure-cli/internal/api"
	"github.com/dgilperez/sure-cli/internal/insights"
	"github.com/dgilperez/sure-cli/internal/output"
	"github.com/dgilperez/sure-cli/internal/plan"
	"github.com/spf13/cobra"
)

func newStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Financial snapshot (accounts, spend, runway, alerts)",
		Run: func(cmd *cobra.Command, args []string) {
			client := api.New()

			// 1. Get accounts
			var accountsRes map[string]any
			_, err := client.Get("/api/v1/accounts", &accountsRes)
			if err != nil {
				output.Fail("request_failed", err.Error(), nil)
			}

			accounts, _ := accountsRes["accounts"].([]any)
			var totalBalance float64
			var cashBalance float64
			var accountSummaries []map[string]any
			_ = "" // unused vars placeholder

			for _, acc := range accounts {
				a, _ := acc.(map[string]any)
				name := fmt.Sprint(a["name"])
				balance := fmt.Sprint(a["balance"])
				accType := fmt.Sprint(a["account_type"])

				bal, _ := insights.ParseAmountEUR(balance)
				totalBalance += bal

				accountSummaries = append(accountSummaries, map[string]any{
					"name":    name,
					"type":    accType,
					"balance": balance,
				})

				// Track cash accounts for runway
				if accType == "depository" || accType == "checking" || accType == "savings" {
					cashBalance += bal
				}
			}

			// 2. Get recent transactions for spend analysis
			end := time.Now().UTC()
			start := end.AddDate(0, -1, 0) // last month
			txs, err := api.FetchTransactionsWindow(client, start, end, 500)
			if err != nil {
				output.Fail("request_failed", err.Error(), nil)
			}

			// Calculate monthly spend
			var monthlySpend float64
			var monthlyIncome float64
			for _, tx := range txs {
				amt, err := insights.ParseAmountEUR(tx.AmountText)
				if err != nil {
					continue
				}
				if tx.Classification == "expense" {
					monthlySpend += absFloat(amt)
				} else if tx.Classification == "income" {
					monthlyIncome += absFloat(amt)
				}
			}

			// 3. Calculate runway if we have cash balance
			var runwayMonths float64
			if monthlySpend > 0 && cashBalance > 0 {
				runwayMonths = cashBalance / monthlySpend
			}

			// 4. Detect potential issues (alerts)
			var alerts []map[string]any

			// Check for high burn rate
			if monthlyIncome > 0 && monthlySpend > monthlyIncome*1.2 {
				alerts = append(alerts, map[string]any{
					"type":    "high_burn",
					"message": fmt.Sprintf("Spending %.0f%% more than income", ((monthlySpend/monthlyIncome)-1)*100),
				})
			}

			// Check for low runway
			if runwayMonths > 0 && runwayMonths < 3 {
				alerts = append(alerts, map[string]any{
					"type":    "low_runway",
					"message": fmt.Sprintf("Only %.1f months of runway remaining", runwayMonths),
				})
			}

			// 5. Get subscription count
			subTxs, _ := api.FetchTransactionsWindow(client, end.AddDate(0, -6, 0), end, 500)
			subs := insights.DetectSubscriptions(subTxs)
			var monthlySubscriptions float64
			for _, s := range subs {
				if s.AvgPeriodDays > 0 {
					monthlySubscriptions += s.AvgAmount * (30.0 / s.AvgPeriodDays)
				}
			}

			// 6. Budget pacing for current month
			budgetResult, _ := plan.ComputeMonthlyBudget(time.Now(), txs)

			status := map[string]any{
				"as_of": time.Now().UTC().Format(time.RFC3339),
				"accounts": map[string]any{
					"count":         len(accounts),
					"total_balance": fmt.Sprintf("€%.2f", totalBalance),
					"cash_balance":  fmt.Sprintf("€%.2f", cashBalance),
					"list":          accountSummaries,
				},
				"monthly": map[string]any{
					"income":        fmt.Sprintf("€%.2f", monthlyIncome),
					"expenses":      fmt.Sprintf("€%.2f", monthlySpend),
					"net":           fmt.Sprintf("€%.2f", monthlyIncome-monthlySpend),
					"subscriptions": fmt.Sprintf("€%.2f", monthlySubscriptions),
				},
				"runway": map[string]any{
					"months":       runwayMonths,
					"cash_balance": fmt.Sprintf("€%.2f", cashBalance),
					"burn_rate":    fmt.Sprintf("€%.2f/month", monthlySpend),
				},
				"budget_pacing": map[string]any{
					"month":        budgetResult.Month,
					"days_elapsed": budgetResult.DaysElapsed,
					"spent":        fmt.Sprintf("€%.2f", budgetResult.Spent),
					"projected":    fmt.Sprintf("€%.2f", budgetResult.Projected),
					"avg_per_day":  fmt.Sprintf("€%.2f", budgetResult.AvgPerDay),
				},
				"alerts":      alerts,
				"alert_count": len(alerts),
			}

			_ = output.Print(format, output.Envelope{Data: status, Meta: &output.Meta{Status: 200}})
		},
	}
	return cmd
}

func absFloat(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
