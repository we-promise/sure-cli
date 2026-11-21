package root

import (
	"encoding/json"
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

func TestTransactionsList_RejectsFiltersSureWouldMisread(t *testing.T) {
	// Sure's apply_filters runs dates through Date.parse (a bad one is a 500,
	// 05/01/2026 is 5 January), amounts through to_f (abc is 0) and ignores
	// an unknown type, so each is checked before any request.
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits++ }))
	t.Cleanup(srv.Close)
	cfg := apiKeyConfig(t, t.TempDir(), srv.URL)
	for args, want := range map[string]string{
		"--start-date 2026-02-30":                       "--start-date: invalid date",
		"--from 05/01/2026":                             "--start-date: invalid date",
		"--end-date 2026-13-01":                         "--end-date: invalid date",
		"--start-date 2026-03-01 --end-date 2026-02-01": "--start-date must be on or before --end-date",
		"--min-amount abc":                              "--min-amount must be a decimal number",
		"--max-amount 1,5":                              "--max-amount must be a decimal number",
		"--type incme":                                  "--type must be income or expense",
	} {
		child, stdout := startCLI(t, append([]string{"--config", cfg, "transactions", "list"}, strings.Fields(args)...)...)
		_ = child.Wait()
		var env struct {
			Error struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal(stdout.Bytes(), &env); err != nil || env.Error.Code != "validation_failed" || !strings.Contains(env.Error.Message, want) {
			t.Errorf("%s: got %s, want validation_failed %q", args, stdout, want)
		}
	}
	if hits != 0 {
		t.Errorf("sent %d requests for rejected filters", hits)
	}
}
