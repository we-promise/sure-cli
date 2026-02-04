package root

import (
	"fmt"
	"net/url"
	"strconv"
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

			txs, err := fetchTransactionsWindow(start, end)
			if err != nil {
				output.Fail("request_failed", err.Error(), nil)
			}
			cands := insights.DetectSubscriptions(txs)
			_ = output.Print(format, output.Envelope{Data: map[string]any{
				"window":     map[string]any{"start": start.Format("2006-01-02"), "end": end.Format("2006-01-02")},
				"candidates": cands,
			}})
		},
	}
	cmd.Flags().IntVar(&months, "months", 6, "lookback months")
	return cmd
}

func fetchTransactionsWindow(start, end time.Time) ([]insights.Transaction, error) {
	client := api.New()
	page := 1
	perPage := 100
	var all []insights.Transaction

	for {
		q := url.Values{}
		q.Set("page", fmt.Sprintf("%d", page))
		q.Set("per_page", fmt.Sprintf("%d", perPage))
		q.Set("start_date", start.Format("2006-01-02"))
		q.Set("end_date", end.Format("2006-01-02"))
		path := "/api/v1/transactions?" + q.Encode()

		var res map[string]any
		r, err := client.Get(path, &res)
		if err != nil {
			return nil, err
		}
		if r.StatusCode() >= 400 {
			return nil, fmt.Errorf("status %d", r.StatusCode())
		}

		items, _ := res["transactions"].([]any)
		for _, it := range items {
			m, _ := it.(map[string]any)
			tx := insights.Transaction{
				ID:             fmt.Sprint(m["id"]),
				Name:           fmt.Sprint(m["name"]),
				Classification: fmt.Sprint(m["classification"]),
				AmountText:     fmt.Sprint(m["amount"]),
				Currency:       fmt.Sprint(m["currency"]),
			}
			if d, ok := m["date"].(string); ok {
				if tt, err := time.Parse("2006-01-02", d); err == nil {
					tx.Date = tt
				}
			}
			if am, ok := m["account"].(map[string]any); ok {
				tx.AccountName = fmt.Sprint(am["name"])
			}
			all = append(all, tx)
		}

		pg, _ := res["pagination"].(map[string]any)
		if pg == nil {
			break
		}
		// total_pages might be float64
		totalPages := int(asFloat(pg["total_pages"]))
		if totalPages <= 0 || page >= totalPages {
			break
		}
		page++
	}

	return all, nil
}

func asFloat(v any) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case int:
		return float64(t)
	case int64:
		return float64(t)
	case string:
		f, _ := strconv.ParseFloat(t, 64)
		return f
	default:
		return 0
	}
}
