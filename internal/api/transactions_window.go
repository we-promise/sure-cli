package api

import (
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	errs "github.com/we-promise/sure-cli/internal/errors"
	"github.com/we-promise/sure-cli/internal/models"
)

// FetchTransactionsWindow pulls the transactions within [start,end] that count
// toward spend and income totals. Use it for any aggregate. It drops what
// Sure's income statement drops and the API lets a client see: rows that are
// not models.Transaction.Reportable, and rows on accounts that are not
// draft or active or are excluded from reports (ReportableAccountIDs).
//
// It approximates Sure's totals rather than reproducing them: the API does
// not expose the per-user include_in_finances scoping or which accounts are
// tax-advantaged (401k, IRA, ISA, ...), both of which Sure leaves out, nor
// the row-level gaps listed on models.Transaction.Reportable.
func FetchTransactionsWindow(client *Client, start, end time.Time, perPage int) ([]models.Transaction, error) {
	accounts, r, err := FetchAllAccounts(client, false)
	if ce := CheckResponse(r, err); ce != nil {
		return nil, ce
	}
	reportableAccounts := ReportableAccountIDs(accounts)

	all, err := FetchAllTransactionsWindow(client, start, end, perPage)
	if err != nil {
		return nil, err
	}
	reportable := all[:0]
	for _, tx := range all {
		if tx.Reportable() && reportableAccounts[tx.AccountID] {
			reportable = append(reportable, tx)
		}
	}
	return reportable, nil
}

// ReportableAccountIDs returns the ids of the accounts Sure's income
// statement reads, given GET /api/v1/accounts without include_disabled
// (draft and active accounts only): those not excluded from reports.
func ReportableAccountIDs(accounts []map[string]any) map[string]bool {
	ids := make(map[string]bool, len(accounts))
	for _, a := range accounts {
		if excluded, _ := a["exclude_from_reports"].(bool); !excluded {
			ids[fmt.Sprint(a["id"])] = true
		}
	}
	return ids
}

// FetchAllTransactionsWindow pulls every transaction within [start,end] by
// paging the Sure API, split parents and excluded rows included (flagged).
// Every failure is a *errs.CLIError, so callers can report it as is.
func FetchAllTransactionsWindow(client *Client, start, end time.Time, perPage int) ([]models.Transaction, error) {
	if perPage <= 0 || perPage > MaxPerPage {
		perPage = MaxPerPage
	}
	q := url.Values{}
	q.Set("start_date", start.Format("2006-01-02"))
	q.Set("end_date", end.Format("2006-01-02"))

	var all []models.Transaction
	r, err := walkPages(client, "/api/v1/transactions", q, perPage, func(body []byte) error {
		var res transactionsPage
		if err := json.Unmarshal(body, &res); err != nil {
			return errs.Wrap(errs.CodeInvalidResponse, "unreadable transactions page", err)
		}
		for _, item := range res.Transactions {
			tx, err := item.toModel()
			if err != nil {
				return errs.Wrap(errs.CodeInvalidResponse, fmt.Sprintf("transaction %s has an unreadable amount", item.ID), err)
			}
			all = append(all, tx)
		}
		return nil
	})
	if ce := CheckResponse(r, err); ce != nil {
		return nil, ce
	}
	return uniqueByID(all, func(tx models.Transaction) string { return tx.ID }), nil
}

// transactionsPage is the subset of GET /api/v1/transactions the CLI reads,
// as rendered by Sure's api/v1/transactions/_transaction.json.jbuilder.
type transactionsPage struct {
	Transactions []apiTransaction `json:"transactions"`
}

type apiTransaction struct {
	ID                string       `json:"id"`
	EntryID           string       `json:"entry_id"`
	Date              string       `json:"date"`
	Amount            string       `json:"amount"`
	AmountDecimal     *string      `json:"amount_decimal"`
	SignedAmountCents *json.Number `json:"signed_amount_cents"`
	Currency          string       `json:"currency"`
	Name              string       `json:"name"`
	Classification    string       `json:"classification"`
	Kind              string       `json:"kind"`
	ExternalID        string       `json:"external_id"`
	Source            string       `json:"source"`
	UserModified      bool         `json:"user_modified"`
	Excluded          bool         `json:"excluded"`
	SplitParent       bool         `json:"split_parent"`
	Account           *struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"account"`
	Category *struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"category"`
	Merchant *struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"merchant"`
}

func (a apiTransaction) toModel() (models.Transaction, error) {
	amount, amountDecimal, err := models.ResolveSignedAmount(models.AmountFields{
		Decimal:        a.AmountDecimal,
		SignedCents:    a.SignedAmountCents,
		Display:        a.Amount,
		Currency:       a.Currency,
		Classification: a.Classification,
	})
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
		tx.AccountID = a.Account.ID
		tx.AccountName = a.Account.Name
	}
	if a.Category != nil {
		tx.CategoryName = a.Category.Name
		tx.CategoryID = a.Category.ID
	}
	if a.Merchant != nil {
		tx.MerchantID = a.Merchant.ID
		tx.MerchantName = a.Merchant.Name
	}
	return tx, nil
}
