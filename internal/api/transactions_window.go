package api

import (
	"fmt"
	"net/url"
	"time"

	"github.com/we-promise/sure-cli/internal/models"
)

// FetchTransactionsWindow pulls all transactions within [start,end] by paging the Sure API.
// It returns an agent-friendly typed slice (no map[string]any).
func FetchTransactionsWindow(client *Client, start, end time.Time, perPage int) ([]models.Transaction, error) {
	if perPage <= 0 {
		perPage = 100
	}

	page := 1
	var all []models.Transaction

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
			return nil, fmt.Errorf("request failed: status %d", r.StatusCode())
		}

		items, _ := res["transactions"].([]any)
		for _, it := range items {
			m, _ := it.(map[string]any)
			tx := models.Transaction{
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
			if cm, ok := m["category"].(map[string]any); ok {
				tx.CategoryName = fmt.Sprint(cm["name"])
				tx.CategoryID = fmt.Sprint(cm["id"])
			}
			if mm, ok := m["merchant"].(map[string]any); ok {
				tx.MerchantName = fmt.Sprint(mm["name"])
			}
			all = append(all, tx)
		}

		pg, _ := res["pagination"].(map[string]any)
		if pg == nil {
			break
		}
		totalPages := asInt(pg["total_pages"])
		if totalPages <= 0 || page >= totalPages {
			break
		}
		page++
	}

	return all, nil
}

func asInt(v any) int {
	switch t := v.(type) {
	case float64:
		return int(t)
	case int:
		return t
	case int64:
		return int(t)
	case string:
		// best effort
		var n int
		_, _ = fmt.Sscanf(t, "%d", &n)
		return n
	default:
		return 0
	}
}
