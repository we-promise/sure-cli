package api

import (
	"fmt"
	"net/url"
	"time"

	"github.com/we-promise/sure-cli/internal/models"
)

// FetchTransactionsWindow pulls the transactions within [start,end] that count
// toward spend and income totals (see models.Transaction.Reportable): split
// parents, excluded entries and budget-excluded kinds are dropped, as Sure's
// own reports drop them. Use it for any aggregate.
func FetchTransactionsWindow(client *Client, start, end time.Time, perPage int) ([]models.Transaction, error) {
	all, err := FetchAllTransactionsWindow(client, start, end, perPage)
	if err != nil {
		return nil, err
	}
	reportable := all[:0]
	for _, tx := range all {
		if tx.Reportable() {
			reportable = append(reportable, tx)
		}
	}
	return reportable, nil
}

// FetchAllTransactionsWindow pulls every transaction within [start,end] by
// paging the Sure API, split parents and excluded rows included (flagged).
func FetchAllTransactionsWindow(client *Client, start, end time.Time, perPage int) ([]models.Transaction, error) {
	if perPage <= 0 {
		perPage = 100
	}

	var all []models.Transaction
	for page := 1; ; page++ {
		q := url.Values{}
		q.Set("page", fmt.Sprintf("%d", page))
		q.Set("per_page", fmt.Sprintf("%d", perPage))
		q.Set("start_date", start.Format("2006-01-02"))
		q.Set("end_date", end.Format("2006-01-02"))

		var res transactionsPage
		r, err := client.Get("/api/v1/transactions?"+q.Encode(), &res)
		if err != nil {
			return nil, err
		}
		if r.StatusCode() >= 400 {
			return nil, fmt.Errorf("request failed: status %d", r.StatusCode())
		}

		for _, item := range res.Transactions {
			tx, err := item.toModel()
			if err != nil {
				return nil, fmt.Errorf("transaction %s: %w", item.ID, err)
			}
			all = append(all, tx)
		}

		if res.Pagination == nil || page >= res.Pagination.TotalPages {
			break
		}
	}

	return all, nil
}

// transactionsPage is the subset of GET /api/v1/transactions the CLI reads,
// as rendered by Sure's api/v1/transactions/_transaction.json.jbuilder.
type transactionsPage struct {
	Transactions []apiTransaction `json:"transactions"`
	Pagination   *struct {
		TotalPages int `json:"total_pages"`
	} `json:"pagination"`
}

type apiTransaction struct {
	ID             string  `json:"id"`
	EntryID        string  `json:"entry_id"`
	Date           string  `json:"date"`
	Amount         string  `json:"amount"`
	AmountDecimal  *string `json:"amount_decimal"`
	Currency       string  `json:"currency"`
	Name           string  `json:"name"`
	Classification string  `json:"classification"`
	Kind           string  `json:"kind"`
	ExternalID     string  `json:"external_id"`
	Source         string  `json:"source"`
	UserModified   bool    `json:"user_modified"`
	Excluded       bool    `json:"excluded"`
	SplitParent    bool    `json:"split_parent"`
	Account        *struct {
		Name string `json:"name"`
	} `json:"account"`
	Category *struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"category"`
	Merchant *struct {
		Name string `json:"name"`
	} `json:"merchant"`
}

func (a apiTransaction) toModel() (models.Transaction, error) {
	amount, amountDecimal, err := models.ResolveSignedAmount(a.AmountDecimal, a.Amount, a.Classification)
	if err != nil {
		return models.Transaction{}, err
	}
	tx := models.Transaction{
		ID:             a.ID,
		EntryID:        a.EntryID,
		Name:           a.Name,
		Classification: a.Classification,
		Kind:           a.Kind,
		Amount:         amount,
		AmountDecimal:  amountDecimal,
		AmountText:     a.Amount,
		Currency:       a.Currency,
		ExternalID:     a.ExternalID,
		Source:         a.Source,
		UserModified:   a.UserModified,
		Excluded:       a.Excluded,
		SplitParent:    a.SplitParent,
	}
	if d, err := time.Parse("2006-01-02", a.Date); err == nil {
		tx.Date = d
	}
	if a.Account != nil {
		tx.AccountName = a.Account.Name
	}
	if a.Category != nil {
		tx.CategoryName = a.Category.Name
		tx.CategoryID = a.Category.ID
	}
	if a.Merchant != nil {
		tx.MerchantName = a.Merchant.Name
	}
	return tx, nil
}
