package root

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/we-promise/sure-cli/internal/api"
	"github.com/we-promise/sure-cli/internal/insights"
	"github.com/we-promise/sure-cli/internal/output"
	"github.com/we-promise/sure-cli/internal/plan"
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
			primaryCurrency := ""

			for _, acc := range accounts {
				a, _ := acc.(map[string]any)
				name := fmt.Sprint(a["name"])
				balance := fmt.Sprint(a["balance"])
				cashBalanceText := fmt.Sprint(a["cash_balance"])
				accType := fmt.Sprint(a["account_type"])
				currency := fmt.Sprint(a["currency"])
				if primaryCurrency == "" && currency != "" && currency != "<nil>" {
					primaryCurrency = currency
				}

				bal, _ := amountFromAPI(a, "balance", "balance_cents")
				totalBalance += bal

				accountSummaries = append(accountSummaries, map[string]any{
					"name":         name,
					"type":         accType,
					"balance":      balance,
					"cash_balance": cashBalanceText,
					"currency":     currency,
				})

				// Track cash accounts for runway
				if accType == "depository" || accType == "checking" || accType == "savings" {
					cash, ok := amountFromAPIOK(a, "cash_balance", "cash_balance_cents")
					if !ok {
						cash = bal
					}
					cashBalance += cash
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
				amt, err := insights.ParseAmount(tx.AmountText)
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
					"total_balance": formatMoneyValue(totalBalance, primaryCurrency),
					"cash_balance":  formatMoneyValue(cashBalance, primaryCurrency),
					"list":          accountSummaries,
				},
				"monthly": map[string]any{
					"income":        formatMoneyValue(monthlyIncome, primaryCurrency),
					"expenses":      formatMoneyValue(monthlySpend, primaryCurrency),
					"net":           formatMoneyValue(monthlyIncome-monthlySpend, primaryCurrency),
					"subscriptions": formatMoneyValue(monthlySubscriptions, primaryCurrency),
				},
				"runway": map[string]any{
					"months":       runwayMonths,
					"cash_balance": formatMoneyValue(cashBalance, primaryCurrency),
					"burn_rate":    fmt.Sprintf("%s/month", formatMoneyValue(monthlySpend, primaryCurrency)),
				},
				"budget_pacing": map[string]any{
					"month":        budgetResult.Month,
					"days_elapsed": budgetResult.DaysElapsed,
					"spent":        formatMoneyValue(budgetResult.Spent, primaryCurrency),
					"projected":    formatMoneyValue(budgetResult.Projected, primaryCurrency),
					"avg_per_day":  formatMoneyValue(budgetResult.AvgPerDay, primaryCurrency),
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

func amountFromAPI(m map[string]any, formattedKey, centsKey string) (float64, error) {
	amount, ok := amountFromAPIOK(m, formattedKey, centsKey)
	if ok {
		return amount, nil
	}
	return 0, fmt.Errorf("missing amount")
}

func amountFromAPIOK(m map[string]any, formattedKey, centsKey string) (float64, bool) {
	switch v := m[centsKey].(type) {
	case float64:
		return v / 100.0, true
	case int:
		return float64(v) / 100.0, true
	case int64:
		return float64(v) / 100.0, true
	case string:
		var n float64
		if _, err := fmt.Sscanf(v, "%f", &n); err == nil {
			return n / 100.0, true
		}
	}
	text := fmt.Sprint(m[formattedKey])
	if text == "" || text == "<nil>" {
		return 0, false
	}
	amount, err := insights.ParseAmount(text)
	return amount, err == nil
}

func formatMoneyValue(value float64, currency string) string {
	if currency == "" || currency == "<nil>" {
		return fmt.Sprintf("%.2f", value)
	}
	return fmt.Sprintf("%.2f %s", value, currency)
}
