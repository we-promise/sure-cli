package api

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/go-resty/resty/v2"

	errs "github.com/we-promise/sure-cli/internal/errors"
)

// MaxPerPage is the largest page Sure serves (Api::V1::BaseController#safe_per_page_param).
const MaxPerPage = 100

// MaxPages bounds a page walk: a total_pages beyond it (100k rows at
// MaxPerPage) is taken as a broken pagination block, not a reason to loop.
const MaxPages = 1000

// walkPages GETs path one page at a time with q plus page/per_page, handing
// each response body to visit. It follows the total_pages Sure reports on the
// first page (a list that grows mid-walk does not extend it) and stops early
// at a page with no rows. It stops at the first transport error, HTTP status
// >= 400 or visit error and returns that response so callers can report it
// as usual.
func walkPages(c *Client, path string, q url.Values, perPage int, visit func(body []byte) error) (*resty.Response, error) {
	query := url.Values{}
	for k, v := range q {
		query[k] = v
	}
	query.Set("per_page", strconv.Itoa(perPage))

	totalPages := 0
	for page := 1; ; page++ {
		query.Set("page", strconv.Itoa(page))
		r, err := c.Get(path+"?"+query.Encode(), nil)
		if err != nil || r.StatusCode() >= 400 {
			return r, err
		}
		var body map[string]json.RawMessage
		if err := json.Unmarshal(r.Body(), &body); err != nil {
			return r, errs.InvalidResponse(r.StatusCode(), r.Body(), r.Header(), err)
		}
		if err := visit(r.Body()); err != nil {
			return r, err
		}
		var meta struct {
			TotalPages int `json:"total_pages"`
		}
		raw, paginated := body["pagination"]
		if paginated {
			if err := json.Unmarshal(raw, &meta); err != nil {
				return r, errs.Wrap(errs.CodeInvalidResponse, "unreadable pagination in "+path, err)
			}
		}
		if !paginated || !hasRows(body) {
			return r, nil
		}
		if page == 1 {
			totalPages = meta.TotalPages
			if totalPages > MaxPages {
				return r, errs.New(errs.CodeInvalidResponse, fmt.Sprintf("%s reports %d pages, more than the %d the CLI walks", path, totalPages, MaxPages)).
					WithDetails(map[string]any{"total_pages": totalPages})
			}
		}
		if page >= totalPages {
			return r, nil
		}
	}
}

// hasRows reports whether any top-level list in a Sure index body is
// non-empty. Index bodies hold one list (e.g. "accounts") plus pagination.
func hasRows(body map[string]json.RawMessage) bool {
	for _, raw := range body {
		var rows []json.RawMessage
		if json.Unmarshal(raw, &rows) == nil && len(rows) > 0 {
			return true
		}
	}
	return false
}

// uniqueByID keeps the first item for each id. Offset paging over a list
// that gains rows mid-walk serves a row twice; items with no id are kept.
func uniqueByID[T any](items []T, id func(T) string) []T {
	seen := make(map[string]bool, len(items))
	out := items[:0]
	for _, it := range items {
		k := id(it)
		if k != "" {
			if seen[k] {
				continue
			}
			seen[k] = true
		}
		out = append(out, it)
	}
	return out
}

// FetchAllPages walks a paginated Sure index endpoint at MaxPerPage and
// returns every element of the `key` array across all pages, each id once,
// as an empty (never nil) slice when there are none.
//
// The last response is always returned so callers can hand it to their usual
// error reporting: on an HTTP status >= 400 paging stops, items is nil and
// err is nil. err is set for transport failures and, as an invalid_response
// CLIError, for bodies that cannot be decoded.
func FetchAllPages(c *Client, path string, q url.Values, key string) ([]map[string]any, *resty.Response, error) {
	all := []map[string]any{}
	r, err := walkPages(c, path, q, MaxPerPage, func(body []byte) error {
		var page map[string]json.RawMessage
		if err := json.Unmarshal(body, &page); err != nil {
			return errs.Wrap(errs.CodeInvalidResponse, "unreadable response from "+path, err)
		}
		var items []map[string]any
		if raw, ok := page[key]; ok {
			if err := json.Unmarshal(raw, &items); err != nil {
				return errs.Wrap(errs.CodeInvalidResponse, fmt.Sprintf("unreadable %q list from %s", key, path), err)
			}
		}
		all = append(all, items...)
		return nil
	})
	if err != nil || r == nil || r.StatusCode() >= 400 {
		return nil, r, err
	}
	return uniqueByID(all, func(m map[string]any) string {
		id, _ := m["id"].(string)
		return id
	}), r, nil
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

	budgets := []map[string]any{}
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
