package api

import (
	"fmt"
	"net/url"

	"github.com/go-resty/resty/v2"
)

// MaxPerPage is the largest page Sure serves (Api::V1::BaseController#safe_per_page_param).
const MaxPerPage = 100

// FetchAllPages walks a paginated Sure index endpoint at MaxPerPage and
// returns every element of the `key` array across all pages.
//
// The last response is always returned so callers can hand it to their usual
// error reporting: on an HTTP status >= 400 paging stops, items is nil and
// err is nil. err is only set for transport failures.
func FetchAllPages(c *Client, path string, q url.Values, key string) ([]map[string]any, *resty.Response, error) {
	query := url.Values{}
	for k, v := range q {
		query[k] = v
	}
	query.Set("per_page", fmt.Sprint(MaxPerPage))

	var all []map[string]any
	for page := 1; ; page++ {
		query.Set("page", fmt.Sprint(page))
		var res map[string]any
		r, err := c.Get(path+"?"+query.Encode(), &res)
		if err != nil || r.StatusCode() >= 400 {
			return nil, r, err
		}
		items, _ := res[key].([]any)
		for _, it := range items {
			if m, ok := it.(map[string]any); ok {
				all = append(all, m)
			}
		}
		pg, _ := res["pagination"].(map[string]any)
		if pg == nil || page >= asInt(pg["total_pages"]) {
			return all, r, nil
		}
	}
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
