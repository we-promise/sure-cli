package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"

	errs "github.com/we-promise/sure-cli/internal/errors"
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

func TestFetchAllPages_MalformedBodyIsInvalidResponse(t *testing.T) {
	c := pagesClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"accounts":{"not":"a list"}}`))
	})
	_, _, err := FetchAllAccounts(c, false)
	var ce *errs.CLIError
	if !errors.As(err, &ce) || ce.Code != errs.CodeInvalidResponse {
		t.Fatalf("err = %v, want CLIError %q", err, errs.CodeInvalidResponse)
	}
}

func TestFetchAllPages_StopsOnEmptyPage(t *testing.T) {
	// A total_pages that never ends must not walk forever.
	calls := 0
	c := pagesClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("page") == "1" {
			_, _ = w.Write([]byte(`{"accounts":[{"id":"a"}],"pagination":{"page":1,"total_pages":500}}`))
			return
		}
		_, _ = w.Write([]byte(`{"accounts":[],"pagination":{"total_pages":500}}`))
	})
	items, _, err := FetchAllPages(c, "/api/v1/accounts", nil, "accounts")
	if err != nil || len(items) != 1 || calls != 2 {
		t.Fatalf("items=%v calls=%d err=%v", items, calls, err)
	}
}

func TestFetchAllPages_FollowsTheLatestTotalPages(t *testing.T) {
	// t6 lands at the head after page 1 is served: t4 is served twice and
	// t2, which existed before the walk began, is pushed onto a third page.
	// total_count grows with the insert, which skips no row.
	ids := []string{"t5", "t4", "t3", "t2"}
	c := pagesClient(t, func(w http.ResponseWriter, r *http.Request) {
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		rows := ids[min(2*(page-1), len(ids)):min(2*page, len(ids))]
		body, _ := json.Marshal(map[string]any{"accounts": idRows(rows...), "pagination": map[string]any{"total_pages": (len(ids) + 1) / 2, "total_count": len(ids)}})
		if page == 1 {
			ids = append([]string{"t6"}, ids...)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	})
	items, _, err := FetchAllPages(c, "/api/v1/accounts", nil, "accounts")
	if err != nil {
		t.Fatal(err)
	}
	if got := itemIDs(items); fmt.Sprint(got) != "[t5 t4 t3 t2]" {
		t.Fatalf("ids = %v, want every row that existed when the walk began", got)
	}
}

func TestFetchAllPages_InsertMidWalkGrowsTotalCountWithoutFailing(t *testing.T) {
	// t6 lands at the head after page 1 and the page count stays 2: t4 is
	// served twice and t2 not at all, yet t2 is a row the walk began
	// without, so every row that existed then is read.
	ids := []string{"t5", "t4", "t3"}
	c := pagesClient(t, func(w http.ResponseWriter, r *http.Request) {
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		rows := ids[min(2*(page-1), len(ids)):min(2*page, len(ids))]
		body, _ := json.Marshal(map[string]any{"accounts": idRows(rows...), "pagination": map[string]any{"total_pages": (len(ids) + 1) / 2, "total_count": len(ids)}})
		if page == 1 {
			ids = append([]string{"t6"}, ids...)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	})
	items, _, err := FetchAllPages(c, "/api/v1/accounts", nil, "accounts")
	if err != nil {
		t.Fatalf("an insert mid-walk failed the walk: %v", err)
	}
	if got := itemIDs(items); fmt.Sprint(got) != "[t5 t4 t3]" {
		t.Fatalf("ids = %v", got)
	}
}

func TestWalkPages_StopsAtTheCapWhenTotalPagesKeepsGrowing(t *testing.T) {
	calls := 0
	c := pagesClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"accounts":[{"id":"a%d"}],"pagination":{"total_pages":%d}}`, calls, calls+1)
	})
	_, err := walkPages(c, "/api/v1/accounts", nil, MaxPerPage, 5, func([]byte) error { return nil })
	assertCode(t, err, errs.CodeTooManyPages)
	if calls != 5 {
		t.Fatalf("calls = %d, want 5 (the cap)", calls)
	}
}

func TestFetchAllPages_PageLargerThanPerPageIsInvalidResponse(t *testing.T) {
	rows := make([]string, MaxPerPage+1)
	for i := range rows {
		rows[i] = fmt.Sprint(i)
	}
	c := pagesClient(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := json.Marshal(map[string]any{"accounts": idRows(rows...), "pagination": map[string]any{"total_pages": 1}})
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	})
	_, _, err := FetchAllPages(c, "/api/v1/accounts", nil, "accounts")
	assertCode(t, err, errs.CodeInvalidResponse)
}

func TestFetchAllPages_ListThatChangesMidWalkIsListChanged(t *testing.T) {
	for name, pages := range map[string][2]string{
		// A row deleted after page 1 pulls c back onto page 1, so c is
		// never served.
		"total_count drops": {
			`{"accounts":[{"id":"a"},{"id":"b"}],"pagination":{"total_pages":2,"total_count":4}}`,
			`{"accounts":[{"id":"d"}],"pagination":{"total_pages":2,"total_count":3}}`,
		},
		// Sure overflows to the last page: a delete serves it again.
		"rows read fall short of the first total_count": {
			`{"accounts":[{"id":"a"},{"id":"b"}],"pagination":{"total_pages":2,"total_count":4}}`,
			`{"accounts":[{"id":"b"},{"id":"c"}],"pagination":{"total_pages":2,"total_count":4}}`,
		},
	} {
		t.Run(name, func(t *testing.T) {
			c := pagesClient(t, func(w http.ResponseWriter, r *http.Request) {
				page, _ := strconv.Atoi(r.URL.Query().Get("page"))
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(pages[page-1]))
			})
			_, _, err := FetchAllPages(c, "/api/v1/accounts", nil, "accounts")
			ce := assertCode(t, err, errs.CodeListChanged)
			if ce.Details["hint"] == nil {
				t.Fatalf("details = %v, want a hint to retry", ce.Details)
			}
		})
	}
}

func TestFetchAllPages_FailsEarlyWhenTheRateLimitCannotCoverTheWalk(t *testing.T) {
	for remaining, wantCalls := range map[string]int{"3": 1, "4": 5} {
		calls := 0
		c := pagesClient(t, func(w http.ResponseWriter, r *http.Request) {
			calls++
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-RateLimit-Limit", "100")
			w.Header().Set("X-RateLimit-Remaining", remaining)
			w.Header().Set("X-RateLimit-Reset", "1800")
			_, _ = fmt.Fprintf(w, `{"accounts":[{"id":"a%d"}],"pagination":{"total_pages":5}}`, calls)
		})
		_, _, err := FetchAllPages(c, "/api/v1/accounts", nil, "accounts")
		if calls != wantCalls {
			t.Fatalf("remaining %s: calls = %d, want %d", remaining, calls, wantCalls)
		}
		if wantCalls == 5 {
			if err != nil {
				t.Fatalf("remaining %s: %v", remaining, err)
			}
			continue
		}
		ce := assertCode(t, err, errs.CodeRateLimit)
		if ce.Details["needed_requests"] != 4 || ce.Details["hint"] == nil {
			t.Fatalf("details = %v", ce.Details)
		}
		if rl, _ := ce.Details["rate_limit"].(map[string]string); rl["remaining"] != "3" || rl["reset"] != "1800" {
			t.Fatalf("rate_limit = %v", ce.Details["rate_limit"])
		}
	}
}

func idRows(ids ...string) []map[string]string {
	rows := make([]map[string]string, len(ids))
	for i, id := range ids {
		rows[i] = map[string]string{"id": id}
	}
	return rows
}

func itemIDs(items []map[string]any) []any {
	ids := make([]any, len(items))
	for i, it := range items {
		ids[i] = it["id"]
	}
	return ids
}

func TestFetchAllPages_ImplausiblePageCountIsTooManyPages(t *testing.T) {
	calls := 0
	c := pagesClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"accounts":[{"id":"a"}],"pagination":{"total_pages":2147483647}}`))
	})
	_, _, err := FetchAllPages(c, "/api/v1/accounts", nil, "accounts")
	ce := assertCode(t, err, errs.CodeTooManyPages)
	if ce.Details["total_pages"] != 2147483647 || ce.Details["hint"] == nil {
		t.Fatalf("details = %v", ce.Details)
	}
	if calls != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}
}

func TestFetchAllPages_DropsRowsRepeatedByAShiftingList(t *testing.T) {
	// A row inserted at the head between page requests pushes the last row
	// of page 1 onto page 2.
	c := pagesClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("page") == "1" {
			_, _ = w.Write([]byte(`{"accounts":[{"id":"a5"},{"id":"a4"}],"pagination":{"total_pages":2,"total_count":3}}`))
			return
		}
		_, _ = w.Write([]byte(`{"accounts":[{"id":"a4"},{"id":"a3"}],"pagination":{"total_pages":2,"total_count":4}}`))
	})
	items, _, err := FetchAllPages(c, "/api/v1/accounts", nil, "accounts")
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, it := range items {
		ids = append(ids, it["id"].(string))
	}
	if fmt.Sprint(ids) != "[a5 a4 a3]" {
		t.Fatalf("ids = %v", ids)
	}
}

func TestFetchAllPages_EmptyIndexIsAnEmptyList(t *testing.T) {
	c := pagesClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"accounts":[],"pagination":{"total_pages":0}}`))
	})
	items, _, err := FetchAllPages(c, "/api/v1/accounts", nil, "accounts")
	if err != nil || items == nil {
		t.Fatalf("items=%#v err=%v, want an empty non-nil slice (JSON [] not null)", items, err)
	}
}

func TestUniqueByID_KeepsFirstAndRowsWithoutID(t *testing.T) {
	got := uniqueByID([]string{"a", "", "b", "a", ""}, func(s string) string { return s })
	if fmt.Sprint(got) != "[a  b ]" {
		t.Fatalf("got %q", got)
	}
}
