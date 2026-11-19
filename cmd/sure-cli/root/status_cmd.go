package root

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/we-promise/sure-cli/internal/api"
	"github.com/we-promise/sure-cli/internal/insights"
	"github.com/we-promise/sure-cli/internal/money"
	"github.com/we-promise/sure-cli/internal/output"
	"github.com/we-promise/sure-cli/internal/plan"
)

func newStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Financial snapshot (net worth, accounts, spend, runway, alerts)",
		Run: func(cmd *cobra.Command, args []string) {
			client := api.New()

			// 1. Net worth: Sure's balance sheet is authoritative (FX-converted
			// to the family currency, honours exclude_from_reports).
			var bsRes map[string]any
			r, err := client.Get("/api/v1/balance_sheet", &bsRes)
			checkResponse(r, err)
			bs := parseBalanceSheet(bsRes)
			primaryCurrency := bs.Currency

			// 2. Every account across pages, for the native-currency breakdown
			// and the cash that funds runway.
			accounts, r, err := api.FetchAllAccounts(client, false)
			checkResponse(r, err)
			totals, accountSummaries := summarizeAccounts(accounts)

			// Runway can only count cash held in the family currency.
			var cashBalance float64
			if t := totals[primaryCurrency]; t != nil {
				cashBalance = t.Cash
			}
			byCurrency := make(map[string]any, len(totals))
			for cur, t := range totals {
				byCurrency[cur] = map[string]any{
					"assets":      formatMoneyValue(t.Assets, cur),
					"liabilities": formatMoneyValue(t.Liabilities, cur),
					"net_worth":   formatMoneyValue(t.NetWorth(), cur),
					"cash":        formatMoneyValue(t.Cash, cur),
				}
			}

			// 3. Get recent transactions for spend analysis
			end := time.Now().UTC()
			start := end.AddDate(0, -1, 0) // last month
			txs, err := api.FetchTransactionsWindow(client, start, end, 500)
			if err != nil {
				output.Fail("request_failed", err.Error(), nil)
				return
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

			// 4. Calculate runway if we have cash balance
			var runwayMonths float64
			if monthlySpend > 0 && cashBalance > 0 {
				runwayMonths = cashBalance / monthlySpend
			}

			// 5. Detect potential issues (alerts)
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

			// 6. Get subscription count
			subTxs, _ := api.FetchTransactionsWindow(client, end.AddDate(0, -6, 0), end, 500)
			subs := insights.DetectSubscriptions(subTxs)
			var monthlySubscriptions float64
			for _, s := range subs {
				if s.AvgPeriodDays > 0 {
					monthlySubscriptions += s.AvgAmount * (30.0 / s.AvgPeriodDays)
				}
			}

			// 7. Budget pacing for current month
			budgetResult, _ := plan.ComputeMonthlyBudget(time.Now(), txs)

			status := map[string]any{
				"as_of": time.Now().UTC().Format(time.RFC3339),
				"net_worth": map[string]any{
					"currency":    primaryCurrency,
					"net_worth":   formatMoneyValue(bs.NetWorth, primaryCurrency),
					"assets":      formatMoneyValue(bs.Assets, primaryCurrency),
					"liabilities": formatMoneyValue(bs.Liabilities, primaryCurrency),
				},
				"accounts": map[string]any{
					"count":        len(accounts),
					"cash_balance": formatMoneyValue(cashBalance, primaryCurrency),
					"by_currency":  byCurrency,
					"list":         accountSummaries,
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

// balanceSheet is GET /api/v1/balance_sheet in major units of Currency.
type balanceSheet struct {
	Currency                      string
	NetWorth, Assets, Liabilities float64
}

func parseBalanceSheet(res map[string]any) balanceSheet {
	amount := func(key string) float64 {
		m, _ := res[key].(map[string]any)
		v, _ := decimalValue(m["amount"])
		return v
	}
	cur, _ := res["currency"].(string)
	return balanceSheet{
		Currency:    cur,
		NetWorth:    amount("net_worth"),
		Assets:      amount("assets"),
		Liabilities: amount("liabilities"),
	}
}

// currencyTotals aggregates accounts held in one currency, in major units.
type currencyTotals struct {
	Assets, Liabilities, Cash float64
}

func (t currencyTotals) NetWorth() float64 { return t.Assets - t.Liabilities }

// summarizeAccounts totals accounts per native currency the way Sure's
// balance sheet counts them: by classification, skipping accounts marked
// exclude_from_reports. Cash is the cash balance of depository assets.
// No FX conversion happens here; the converted figure is the balance sheet.
func summarizeAccounts(accounts []map[string]any) (map[string]*currencyTotals, []map[string]any) {
	totals := map[string]*currencyTotals{}
	list := make([]map[string]any, 0, len(accounts))
	for _, a := range accounts {
		currency, _ := a["currency"].(string)
		currency = strings.ToUpper(currency)
		accType := fmt.Sprint(a["account_type"])
		classification := fmt.Sprint(a["classification"])
		excluded, _ := a["exclude_from_reports"].(bool)

		list = append(list, map[string]any{
			"name":                 fmt.Sprint(a["name"]),
			"type":                 accType,
			"classification":       classification,
			"balance":              fmt.Sprint(a["balance"]),
			"cash_balance":         fmt.Sprint(a["cash_balance"]),
			"currency":             currency,
			"exclude_from_reports": excluded,
		})
		if excluded {
			continue
		}

		t := totals[currency]
		if t == nil {
			t = &currencyTotals{}
			totals[currency] = t
		}
		bal, _ := amountFromAPIOK(a, "balance", "balance_cents")
		switch classification {
		case "asset":
			t.Assets += bal
			if accType == "depository" {
				cash, ok := amountFromAPIOK(a, "cash_balance", "cash_balance_cents")
				if !ok {
					cash = bal
				}
				t.Cash += cash
			}
		case "liability":
			t.Liabilities += bal
		}
	}
	return totals, list
}

func absFloat(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

// amountFromAPIOK reads an amount in major units, preferring Sure's integer
// minor-unit field (scaled by the currency's minor unit) over parsing the
// locale-formatted string, which is ambiguous ("1.234,56 €").
func amountFromAPIOK(m map[string]any, formattedKey, centsKey string) (float64, bool) {
	if minor, ok := decimalValue(m[centsKey]); ok {
		return money.FromMinor(minor, fmt.Sprint(m["currency"])), true
	}
	text := fmt.Sprint(m[formattedKey])
	if text == "" || text == "<nil>" {
		return 0, false
	}
	amount, err := insights.ParseAmount(text)
	return amount, err == nil
}

// decimalValue reads a JSON number, or a decimal string as Rails renders
// BigDecimal.
func decimalValue(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(n), 64)
		return f, err == nil
	}
	return 0, false
}

func formatMoneyValue(value float64, currency string) string {
	if currency == "" || currency == "<nil>" {
		return fmt.Sprintf("%.2f", value)
	}
	return fmt.Sprintf("%.2f %s", value, currency)
}
