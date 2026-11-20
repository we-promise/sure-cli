package api

import (
	"context"
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

// MaxPages is the default bound on a page walk (100k rows at MaxPerPage).
const MaxPages = 1000

// StreamMaxPages bounds walks whose rows are written out as they arrive
// instead of held in memory (1M rows at MaxPerPage).
const StreamMaxPages = 10000

// walkPages GETs path, cancelled with ctx, one page at a time with q plus
// page/per_page, handing each response body to visit, for at most maxPages
// pages. It walks up to the total_pages the latest page reports, so rows
// pushed onto a new page by an insert mid-walk are still read, and stops at a
// page with no rows. It stops at the first transport error, HTTP status >= 400
// or visit error and returns that response so callers can report it as usual.
// It fails, as a CLIError, on a page with more than perPage rows
// (invalid_response), on more than maxPages pages (too_many_pages), when the
// pages left exceed X-RateLimit-Remaining (rate_limited), and when rows may
// have been skipped (list_changed): offset paging over a list that loses rows
// mid-walk skips one, which shows as a total_count lower than the page before
// or, as Sure serves its last page again for a page past the end (Pagy
// overflow :last_page), fewer rows read, each id once, than the first
// total_count. A row inserted mid-walk only makes a row repeat, which is
// dropped.
func walkPages(ctx context.Context, c *Client, path string, q url.Values, perPage, maxPages int, visit func(body []byte) error) (*resty.Response, error) {
	query := url.Values{}
	for k, v := range q {
		query[k] = v
	}
	query.Set("per_page", strconv.Itoa(perPage))

	seen := map[string]bool{}
	read, firstTotal, lastTotal := 0, -1, -1
	for page := 1; ; page++ {
		query.Set("page", strconv.Itoa(page))
		r, err := c.GetContext(ctx, path+"?"+query.Encode(), nil)
		if err != nil || r.StatusCode() >= 400 {
			return r, err
		}
		var body map[string]json.RawMessage
		if err := json.Unmarshal(r.Body(), &body); err != nil {
			return r, errs.InvalidResponse(r.StatusCode(), r.Body(), r.Header(), err)
		}
		rows := pageRows(body)
		if len(rows) > perPage {
			return r, errs.New(errs.CodeInvalidResponse, fmt.Sprintf("%s served %d rows on a page of %d", path, len(rows), perPage)).
				WithDetails(map[string]any{"status": r.StatusCode(), "rows": len(rows), "per_page": perPage})
		}
		if err := visit(r.Body()); err != nil {
			return r, err
		}
		var meta struct {
			TotalPages int  `json:"total_pages"`
			TotalCount *int `json:"total_count"`
		}
		raw, paginated := body["pagination"]
		if !paginated {
			return r, nil
		}
		if err := json.Unmarshal(raw, &meta); err != nil {
			return r, errs.Wrap(errs.CodeInvalidResponse, "unreadable pagination in "+path, err)
		}
		for _, row := range rows {
			if id := rowID(row); id != "" {
				if seen[id] {
					continue
				}
				seen[id] = true
			}
			read++
		}
		if meta.TotalCount != nil {
			if *meta.TotalCount < lastTotal {
				return r, listChanged(path, read, *meta.TotalCount)
			}
			if firstTotal < 0 {
				firstTotal = *meta.TotalCount
			}
			lastTotal = *meta.TotalCount
		}
		if len(rows) == 0 || page >= meta.TotalPages {
			if read < firstTotal {
				return r, listChanged(path, read, firstTotal)
			}
			return r, nil
		}
		if meta.TotalPages > maxPages {
			return r, errs.New(errs.CodeTooManyPages, fmt.Sprintf("%s has %d pages, more than the %d the CLI reads", path, meta.TotalPages, maxPages)).
				WithDetails(map[string]any{"total_pages": meta.TotalPages, "max_pages": maxPages, "hint": "narrow the query (e.g. a shorter date window), or use family-exports for a full export"})
		}
		if err := checkRateBudget(r, path, meta.TotalPages-page); err != nil {
			return r, err
		}
	}
}

// pageRows returns the rows of a Sure index body: its top-level lists, of
// which there is one (e.g. "accounts") beside pagination.
func pageRows(body map[string]json.RawMessage) []json.RawMessage {
	var all []json.RawMessage
	for _, raw := range body {
		var rows []json.RawMessage
		if json.Unmarshal(raw, &rows) == nil {
			all = append(all, rows...)
		}
	}
	return all
}

// rowID returns a row's id as raw JSON, or "" when it has none.
func rowID(row json.RawMessage) string {
	var r struct {
		ID json.RawMessage `json:"id"`
	}
	if json.Unmarshal(row, &r) != nil || len(r.ID) == 0 || string(r.ID) == "null" {
		return ""
	}
	return string(r.ID)
}

func listChanged(path string, read, totalCount int) *errs.CLIError {
	return errs.New(errs.CodeListChanged, fmt.Sprintf("%s changed while it was being read (%d rows read, Sure counts %d)", path, read, totalCount)).
		WithDetails(map[string]any{"rows_read": read, "total_count": totalCount, "hint": "run the command again"})
}

// checkRateBudget fails before spending the API key's hourly budget on a
// walk that cannot finish: Sure sends X-RateLimit-Remaining to API-key
// callers on a rate-limited (not self-hosted) instance.
func checkRateBudget(r *resty.Response, path string, pagesLeft int) *errs.CLIError {
	remaining, err := strconv.Atoi(r.Header().Get("X-RateLimit-Remaining"))
	if err != nil || pagesLeft <= remaining {
		return nil
	}
	rl := errs.RateLimit(r.Header())
	return errs.New(errs.CodeRateLimit, fmt.Sprintf("%s needs %d more requests but the API key has %d left before its hourly limit resets", path, pagesLeft, remaining)).
		WithDetails(map[string]any{"needed_requests": pagesLeft, "rate_limit": rl, "hint": "wait for the reset, narrow the query, or use family-exports for a full export"})
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
	r, err := walkPages(context.Background(), c, path, q, MaxPerPage, MaxPages, func(body []byte) error {
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
	q.Set("start_date", since.Format(time.DateOnly))
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
