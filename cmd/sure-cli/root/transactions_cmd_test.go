package root

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestTransactionsList_QueryString(t *testing.T) {
	cmd := newTransactionsCmd()

	list, _, err := cmd.Find([]string{"list"})
	if err != nil {
		t.Fatalf("find list subcommand: %v", err)
	}

	// build flags
	_ = list.Flags().Set("start-date", "2026-01-01")
	_ = list.Flags().Set("end-date", "2026-01-31")
	_ = list.Flags().Set("account-id", "1")
	_ = list.Flags().Set("type", "expense")

	// We don't call Run (would hit network), but we can ensure flags exist and are set.
	// This test mainly guards against accidental flag removal/renaming.
	for _, name := range []string{"from", "to", "start-date", "end-date", "account", "account-id", "category-id", "merchant-id", "type", "search", "page", "per-page"} {
		if list.Flags().Lookup(name) == nil {
			t.Fatalf("expected flag %q to exist", name)
		}
	}

	s := list.Flags().FlagUsages()
	if !strings.Contains(s, "from") {
		t.Fatalf("expected from in usage")
	}
}

func TestTransactionsList_SendsSureQuery(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"transactions":[],"pagination":{"page":2,"per_page":50,"total_count":0,"total_pages":0}}`))
	}))
	t.Cleanup(srv.Close)
	apiClientFor(t, srv.URL)

	cmd := newTransactionsCmd()
	// Aliases fill in only what the canonical flag leaves empty.
	cmd.SetArgs([]string{"list", "--from", "2026-01-01", "--end-date", "2026-01-31", "--to", "2026-02-28",
		"--account", "a1", "--category-id", "c1", "--merchant", "m1", "--type", "expense", "--search", "rent",
		"--min-amount", "10", "--max-amount", "20", "--account-ids", "a2,a3", "--tag-ids", "t1", "--tag-ids", " t2 ,",
		"--page", "2", "--per-page", "50"})
	captureStdout(t, func() { _ = cmd.Execute() })

	want := url.Values{
		"start_date": {"2026-01-01"}, "end_date": {"2026-01-31"}, "account_id": {"a1"}, "category_id": {"c1"},
		"merchant_id": {"m1"}, "type": {"expense"}, "search": {"rent"}, "min_amount": {"10"}, "max_amount": {"20"},
		"account_ids[]": {"a2", "a3"}, "tag_ids[]": {"t1", "t2"}, "page": {"2"}, "per_page": {"50"},
	}
	if got != want.Encode() {
		t.Fatalf("query = %s\nwant   %s", got, want.Encode())
	}
}
