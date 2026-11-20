package root

import (
	"fmt"
	"math"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/we-promise/sure-cli/internal/api"
	errs "github.com/we-promise/sure-cli/internal/errors"
	"github.com/we-promise/sure-cli/internal/models"
	"github.com/we-promise/sure-cli/internal/money"
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

			// 3. Income and spending over the last month up to the family's
			// today, as Sure's income statement computes them.
			end := today(familyClock(client)())
			flow := fetchCashFlow(client, lookbackStart(end, 1), end)
			monthlySpend, monthlyIncome := flow.Spending, flow.Income

			// 4. Calculate runway if we have cash balance
			var runwayMonths float64
			if monthlySpend > 0 && cashBalance > 0 {
				runwayMonths = cashBalance / monthlySpend
			}

			// 5. Detect potential issues (alerts)
			alerts := []map[string]any{}

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

			// 6. Recurring outflows Sure tracks, per currency, as a monthly rate.
			series, r, err := api.FetchActiveRecurringTransactions(client)
			checkResponse(r, err)
			subscriptions := recurringMonthlyOutflows(series)
			subscriptionsByCurrency := make(map[string]any, len(subscriptions))
			for cur, v := range subscriptions {
				subscriptionsByCurrency[cur] = formatMoneyValue(v, cur)
			}

			// 7. Pacing of each current budget, in that budget's currency.
			budgets, r, err := api.FetchCurrentBudgets(client, end)
			checkResponse(r, err)
			pacing := make([]map[string]any, 0, len(budgets))
			for _, b := range budgets {
				pacing = append(pacing, budgetPacing(b, end))
			}

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
					"period":        flow.Period,
					"income":        formatMoneyValue(monthlyIncome, primaryCurrency),
					"expenses":      formatMoneyValue(monthlySpend, primaryCurrency),
					"net":           formatMoneyValue(monthlyIncome-monthlySpend, primaryCurrency),
					"subscriptions": formatMoneyValue(subscriptions[primaryCurrency], primaryCurrency),
					// Every currency, family currency included; never summed across.
					"subscriptions_by_currency": subscriptionsByCurrency,
				},
				"runway": map[string]any{
					"months":       runwayMonths,
					"cash_balance": formatMoneyValue(cashBalance, primaryCurrency),
					"burn_rate":    fmt.Sprintf("%s/month", formatMoneyValue(monthlySpend, primaryCurrency)),
				},
				"budget_pacing": pacing,
				"alerts":        alerts,
				"alert_count":   len(alerts),
			}

			printEnvelope(status, nil)
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
// Sure also leaves out accounts a user took out of their finances
// (include_in_finances); the API does not expose that, so they are counted.
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

// cashFlow is the income and spending GET /api/v1/cash_flow?view=sankey
// reports for a period, in the family currency.
type cashFlow struct {
	Period           map[string]any
	Income, Spending float64
}

// fetchCashFlow reads Sure's income and spending over [start, end]: FX
// converted to the family currency, scoped to the accounts the user counts in
// their finances, and netted per category, so a refund lowers its category's
// spending (IncomeStatement::Sankey). It fails on a server older than the
// endpoint.
func fetchCashFlow(client *api.Client, start, end time.Time) cashFlow {
	q := url.Values{"view": {"sankey"}, "start_date": {start.Format(time.DateOnly)}, "end_date": {end.Format(time.DateOnly)}}
	var res map[string]any
	r, err := client.Get(pathWithQuery("/api/v1/cash_flow", q), &res)
	if ce := api.CheckResponse(r, err); ce != nil {
		if ce.Code == errs.CodeNotFound {
			if ce.Details == nil {
				ce.Details = map[string]any{}
			}
			ce.Details["hint"] = "status reads GET /api/v1/cash_flow, which needs Sure v0.7.5 or later; upgrade the server"
		}
		fail(ce)
	}
	sankey, _ := res["sankey"].(map[string]any)
	income, okIncome := decimalValue(sankey["income"])
	spending, okSpending := decimalValue(sankey["spending"])
	if !okIncome || !okSpending {
		fail(errs.New(errs.CodeInvalidResponse, "cash_flow has no decimal sankey.income and sankey.spending").
			WithDetails(map[string]any{"status": r.StatusCode()}))
	}
	period, _ := res["period"].(map[string]any)
	return cashFlow{Period: period, Income: income, Spending: spending}
}

// recurringMonthlyOutflows totals active recurring outflows per currency as a
// monthly rate, the way Sure's RecurringTransaction#monthly_equivalent_amount
// prices them: amount × occurrences per year / 12, a one-off adding nothing.
// Income (negative amounts) and transfers between the family's own accounts
// are not spending (RecurringTransaction.outflows) and are skipped.
func recurringMonthlyOutflows(series []map[string]any) map[string]float64 {
	totals := map[string]float64{}
	for _, s := range series {
		amount, ok := amountFromAPIOK(s, "amount", "amount_cents")
		if !ok || amount <= 0 || s["bill_type"] == "transfer" {
			continue
		}
		rules, _ := s["recurrence_rules"].([]any)
		if isOneOffSeries(s, rules) {
			continue
		}
		currency := strings.ToUpper(fmt.Sprint(s["currency"]))
		totals[currency] += amount * occurrencesPerYear(rules) / 12
	}
	return totals
}

// occurrencesPerYear mirrors RecurringTransaction::Schedule#occurrences_per_year.
// A series without rules is Sure's legacy "monthly on expected_day_of_month".
func occurrencesPerYear(rules []any) float64 {
	if len(rules) == 0 {
		return 12
	}
	var perYear float64
	for _, raw := range rules {
		rule, _ := raw.(map[string]any)
		interval, ok := decimalValue(rule["interval"])
		if !ok || interval <= 0 {
			interval = 1
		}
		switch rule["frequency"] {
		case "weekly":
			perYear += 365.25 / (7 * interval)
		case "monthly":
			perYear += 12 / interval
		case "yearly":
			perYear += 1 / interval
		}
	}
	return perYear
}

// isOneOffSeries mirrors FrequencyPreset.once_schedule?: a single payment is
// stored as one yearly day-of-month rule that ends after one occurrence.
func isOneOffSeries(s map[string]any, rules []any) bool {
	if s["bill_type"] == "installment" || s["end_mode"] != "after_count" || len(rules) != 1 {
		return false
	}
	if count, _ := decimalValue(s["end_after_count"]); count != 1 {
		return false
	}
	rule, _ := rules[0].(map[string]any)
	interval, _ := decimalValue(rule["interval"])
	return rule["frequency"] == "yearly" && interval == 1 && rule["day_of_month"] != nil && rule["weekday"] == nil
}

// budgetPacing summarises a budget (GET /api/v1/budgets/{id}) in its own
// currency: what was budgeted and spent, and how far through its period
// today is. An uninitialized budget has no budgeted figures.
func budgetPacing(b map[string]any, today time.Time) map[string]any {
	currency := fmt.Sprint(b["currency"])
	minor := func(key string) (float64, bool) {
		v, ok := decimalValue(b[key+"_cents"])
		return money.FromMinor(v, currency), ok
	}
	p := map[string]any{
		"id":          b["id"],
		"name":        b["name"],
		"currency":    b["currency"],
		"start_date":  b["start_date"],
		"end_date":    b["end_date"],
		"initialized": b["initialized"],
	}
	for _, key := range []string{"budgeted_spending", "actual_spending", "available_to_spend"} {
		p[key] = nil
		if v, ok := minor(key); ok {
			p[key] = formatMoneyValue(v, currency)
		}
	}
	p["percent_spent"] = nil
	budgeted, okBudgeted := minor("budgeted_spending")
	if actual, ok := minor("actual_spending"); ok && okBudgeted && budgeted > 0 {
		p["percent_spent"] = roundTo1(actual / budgeted * 100)
	}
	p["percent_elapsed"] = nil
	start, errStart := time.Parse(time.DateOnly, fmt.Sprint(b["start_date"]))
	end, errEnd := time.Parse(time.DateOnly, fmt.Sprint(b["end_date"]))
	if errStart == nil && errEnd == nil && !end.Before(start) {
		day := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)
		days := end.Sub(start).Hours()/24 + 1
		elapsed := math.Min(math.Max(day.Sub(start).Hours()/24+1, 0), days)
		p["percent_elapsed"] = roundTo1(elapsed / days * 100)
	}
	return p
}

func roundTo1(v float64) float64 { return math.Round(v*10) / 10 }

// amountFromAPIOK reads an amount in major units, preferring Sure's integer
// minor-unit field (scaled by the currency's minor unit) over the
// locale-formatted string, which can only be read by inference
// (models.ParseDisplayAmount).
func amountFromAPIOK(m map[string]any, formattedKey, centsKey string) (float64, bool) {
	currency := fmt.Sprint(m["currency"])
	if minor, ok := decimalValue(m[centsKey]); ok {
		return money.FromMinor(minor, currency), true
	}
	text, ok := m[formattedKey].(string)
	if !ok {
		return 0, false
	}
	decimal, err := models.ParseDisplayAmount(text, currency)
	if err != nil {
		return 0, false
	}
	amount, err := strconv.ParseFloat(decimal, 64)
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

// formatMoneyValue renders value with the decimals Sure shows for currency
// (money.Precision): "12.50 EUR", "1234 JPY", "0.00012345 BTC".
func formatMoneyValue(value float64, currency string) string {
	if currency == "" || currency == "<nil>" {
		return fmt.Sprintf("%.2f", value)
	}
	return fmt.Sprintf("%.*f %s", money.Precision(currency), value, currency)
}
