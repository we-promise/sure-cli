package api

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/go-resty/resty/v2"
)

// MaxPerPage is the largest page Sure serves (Api::V1::BaseController#safe_per_page_param).
const MaxPerPage = 100

// walkPages GETs path one page at a time with q plus page/per_page, handing
// each response body to visit, until Sure's pagination.total_pages is
// reached. It stops at the first transport error, HTTP status >= 400 or visit
// error and returns that response so callers can report it as usual.
func walkPages(c *Client, path string, q url.Values, perPage int, visit func(body []byte) error) (*resty.Response, error) {
	query := url.Values{}
	for k, v := range q {
		query[k] = v
	}
	query.Set("per_page", strconv.Itoa(perPage))

	for page := 1; ; page++ {
		query.Set("page", strconv.Itoa(page))
		r, err := c.Get(path+"?"+query.Encode(), nil)
		if err != nil || r.StatusCode() >= 400 {
			return r, err
		}
		if err := visit(r.Body()); err != nil {
			return r, err
		}
		var meta struct {
			Pagination *struct {
				TotalPages int `json:"total_pages"`
			} `json:"pagination"`
		}
		if err := json.Unmarshal(r.Body(), &meta); err != nil {
			return r, fmt.Errorf("decode pagination: %w", err)
		}
		if meta.Pagination == nil || page >= meta.Pagination.TotalPages {
			return r, nil
		}
	}
}

// FetchAllPages walks a paginated Sure index endpoint at MaxPerPage and
// returns every element of the `key` array across all pages.
//
// The last response is always returned so callers can hand it to their usual
// error reporting: on an HTTP status >= 400 paging stops, items is nil and
// err is nil. err is set for transport and decoding failures.
func FetchAllPages(c *Client, path string, q url.Values, key string) ([]map[string]any, *resty.Response, error) {
	var all []map[string]any
	r, err := walkPages(c, path, q, MaxPerPage, func(body []byte) error {
		var page map[string]json.RawMessage
		if err := json.Unmarshal(body, &page); err != nil {
			return fmt.Errorf("decode %s: %w", path, err)
		}
		var items []map[string]any
		if raw, ok := page[key]; ok {
			if err := json.Unmarshal(raw, &items); err != nil {
				return fmt.Errorf("decode %s.%s: %w", path, key, err)
			}
		}
		all = append(all, items...)
		return nil
	})
	if err != nil || r == nil || r.StatusCode() >= 400 {
		return nil, r, err
	}
	return all, r, nil
}

// AccountsQuery builds the filters accepted by GET /api/v1/accounts.
// Sure hides disabled accounts unless include_disabled is truthy.
func AccountsQuery(includeDisabled bool) url.Values {
	q := url.Values{}
	if includeDisabled {
		q.Set("include_disabled", "true")
	}
	return q
}

// FetchAllAccounts returns every account visible to the caller, across pages.
func FetchAllAccounts(c *Client, includeDisabled bool) ([]map[string]any, *resty.Response, error) {
	return FetchAllPages(c, "/api/v1/accounts", AccountsQuery(includeDisabled), "accounts")
}

// FetchActiveRecurringTransactions returns every active recurring series
// (GET /api/v1/recurring_transactions?status=active), across pages.
func FetchActiveRecurringTransactions(c *Client) ([]map[string]any, *resty.Response, error) {
	q := url.Values{}
	q.Set("status", "active")
	return FetchAllPages(c, "/api/v1/recurring_transactions", q, "recurring_transactions")
}

// FetchCurrentBudgets returns every visible budget Sure flags `current` (the
// household one and any personal or shared ones), each read through
// GET /api/v1/budgets/{id}: the index leaves out the derived amounts
// (actual_spending, available_to_spend). The index is bounded from the first
// day of the previous month, as a custom month start can open the current
// period there. Responses are handed back as FetchAllPages does.
func FetchCurrentBudgets(c *Client, today time.Time) ([]map[string]any, *resty.Response, error) {
	since := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, -1, 0)
	q := url.Values{}
	q.Set("start_date", since.Format("2006-01-02"))
	index, r, err := FetchAllPages(c, "/api/v1/budgets", q, "budgets")
	if err != nil || r == nil || r.StatusCode() >= 400 {
		return nil, r, err
	}

	var budgets []map[string]any
	for _, b := range index {
		if current, _ := b["current"].(bool); !current {
			continue
		}
		var budget map[string]any
		r, err = c.Get("/api/v1/budgets/"+url.PathEscape(fmt.Sprint(b["id"])), &budget)
		if err != nil || r.StatusCode() >= 400 {
			return nil, r, err
		}
		budgets = append(budgets, budget)
	}
	return budgets, r, nil
}
