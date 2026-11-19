package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

// pagesClient serves h and returns a client pointed at it.
func pagesClient(t *testing.T, h http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return testClient(t, srv.URL)
}

// pagedAccounts serves `total` accounts in Sure's index shape, honouring
// page/per_page the way Api::V1::BaseController#safe_per_page_param does.
func pagedAccounts(t *testing.T, total int, seen *[]url.Values) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/accounts" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		q := r.URL.Query()
		*seen = append(*seen, q)
		page, perPage := 1, 25
		_, _ = fmt.Sscanf(q.Get("page"), "%d", &page)
		_, _ = fmt.Sscanf(q.Get("per_page"), "%d", &perPage)
		pages := (total + perPage - 1) / perPage
		var items []string
		for i := (page - 1) * perPage; i < total && i < page*perPage; i++ {
			items = append(items, fmt.Sprintf(`{"id":"acc-%d"}`, i))
		}
		body := "["
		for i, it := range items {
			if i > 0 {
				body += ","
			}
			body += it
		}
		body += "]"
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"accounts":%s,"pagination":{"page":%d,"per_page":%d,"total_count":%d,"total_pages":%d}}`,
			body, page, perPage, total, pages)
	}
}

func TestFetchAllAccounts_FollowsEveryPageAtMaxPageSize(t *testing.T) {
	var seen []url.Values
	c := pagesClient(t, pagedAccounts(t, 230, &seen))

	items, r, err := FetchAllAccounts(c, false)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if r == nil || r.StatusCode() != 200 {
		t.Fatalf("expected last 200 response, got %v", r)
	}
	if len(items) != 230 {
		t.Fatalf("got %d accounts, want 230", len(items))
	}
	if items[229]["id"] != "acc-229" {
		t.Fatalf("last item = %v", items[229])
	}
	if len(seen) != 3 {
		t.Fatalf("expected 3 requests (100+100+30), got %d", len(seen))
	}
	for i, q := range seen {
		if q.Get("per_page") != "100" {
			t.Fatalf("request %d per_page = %q, want 100 (Sure max)", i, q.Get("per_page"))
		}
		if q.Get("page") != fmt.Sprint(i+1) {
			t.Fatalf("request %d page = %q", i, q.Get("page"))
		}
		if q.Has("include_disabled") {
			t.Fatalf("include_disabled must be omitted when false, got %q", q.Get("include_disabled"))
		}
	}
}

func TestFetchAllAccounts_IncludeDisabledPassthrough(t *testing.T) {
	var seen []url.Values
	c := pagesClient(t, pagedAccounts(t, 1, &seen))

	if _, _, err := FetchAllAccounts(c, true); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got := seen[0].Get("include_disabled"); got != "true" {
		t.Fatalf("include_disabled = %q, want true", got)
	}
}

func TestFetchAllPages_StopsAndReturnsErrorResponse(t *testing.T) {
	calls := 0
	c := pagesClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("page") == "2" {
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"error":"forbidden"}`))
			return
		}
		_, _ = w.Write([]byte(`{"accounts":[{"id":"a"}],"pagination":{"page":1,"total_pages":3}}`))
	})

	items, r, err := FetchAllPages(c, "/api/v1/accounts", nil, "accounts")
	if err != nil {
		t.Fatalf("transport err: %v", err)
	}
	if r == nil || r.StatusCode() != http.StatusForbidden {
		t.Fatalf("expected the 403 response to be handed back for typed error reporting, got %v", r)
	}
	if items != nil {
		t.Fatalf("partial results must not be returned on error, got %v", items)
	}
	if calls != 2 {
		t.Fatalf("expected paging to stop at the failing page, got %d calls", calls)
	}
}

func TestFetchAllPages_NoPaginationBlockIsSinglePage(t *testing.T) {
	calls := 0
	c := pagesClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"accounts":[{"id":"a"},{"id":"b"}]}`))
	})
	items, _, err := FetchAllPages(c, "/api/v1/accounts", url.Values{}, "accounts")
	if err != nil || len(items) != 2 || calls != 1 {
		t.Fatalf("items=%v calls=%d err=%v", items, calls, err)
	}
}

func TestFetchActiveRecurringTransactions_FiltersActiveAcrossPages(t *testing.T) {
	var seen []url.Values
	c := pagesClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/recurring_transactions" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		q := r.URL.Query()
		seen = append(seen, q)
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"recurring_transactions":[{"id":"rt-%s"}],"pagination":{"page":%s,"total_pages":2}}`,
			q.Get("page"), q.Get("page"))
	})

	items, r, err := FetchActiveRecurringTransactions(c)
	if err != nil || r == nil || r.StatusCode() != http.StatusOK {
		t.Fatalf("r=%v err=%v", r, err)
	}
	if len(items) != 2 || items[1]["id"] != "rt-2" {
		t.Fatalf("items = %v", items)
	}
	for i, q := range seen {
		if q.Get("status") != "active" {
			t.Fatalf("request %d status = %q, want active", i, q.Get("status"))
		}
	}
}

func TestFetchCurrentBudgets_ShowsEachCurrentBudget(t *testing.T) {
	var indexQuery url.Values
	var shown []string
	c := pagesClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/budgets":
			indexQuery = r.URL.Query()
			// The index omits derived amounts (include_derived_amounts: false).
			_, _ = w.Write([]byte(`{"budgets":[
				{"id":"b-next","current":false},
				{"id":"b-household","current":true},
				{"id":"b-personal","current":true},
				{"id":"b-prev","current":false}
			],"pagination":{"page":1,"total_pages":1}}`))
		default:
			id := r.URL.Path[len("/api/v1/budgets/"):]
			shown = append(shown, id)
			_, _ = fmt.Fprintf(w, `{"id":%q,"current":true,"actual_spending_cents":1000}`, id)
		}
	})

	today := time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)
	budgets, r, err := FetchCurrentBudgets(c, today)
	if err != nil || r == nil || r.StatusCode() != http.StatusOK {
		t.Fatalf("r=%v err=%v", r, err)
	}
	// A custom month start can open the current period in the previous
	// calendar month, so the index is bounded from its first day.
	if got := indexQuery.Get("start_date"); got != "2026-02-01" {
		t.Fatalf("start_date = %q, want 2026-02-01", got)
	}
	if len(shown) != 2 || shown[0] != "b-household" || shown[1] != "b-personal" {
		t.Fatalf("shown = %v, want the two current budgets", shown)
	}
	if len(budgets) != 2 || budgets[0]["actual_spending_cents"] != float64(1000) {
		t.Fatalf("budgets = %v, want the show payloads", budgets)
	}
}

func TestFetchCurrentBudgets_ReturnsFailingShowResponse(t *testing.T) {
	c := pagesClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/v1/budgets" {
			_, _ = w.Write([]byte(`{"budgets":[{"id":"b1","current":true}]}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"not_found"}`))
	})
	budgets, r, err := FetchCurrentBudgets(c, time.Now())
	if err != nil || r == nil || r.StatusCode() != http.StatusNotFound || budgets != nil {
		t.Fatalf("budgets=%v r=%v err=%v", budgets, r, err)
	}
}
