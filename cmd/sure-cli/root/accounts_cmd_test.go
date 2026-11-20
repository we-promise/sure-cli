package root

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAccountsListFlags(t *testing.T) {
	list := findSub(t, newAccountsCmd(), "list")
	for _, name := range []string{"page", "per-page", "include-disabled", "all"} {
		if list.Flags().Lookup(name) == nil {
			t.Fatalf("expected flag %q on accounts list", name)
		}
	}
	if def := list.Flags().Lookup("include-disabled").DefValue; def != "false" {
		t.Fatalf("include-disabled default = %q, want false (Sure hides disabled accounts by default)", def)
	}
}

func accountsAllServer(t *testing.T, accounts string) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"accounts":%s,"pagination":{"page":1,"per_page":100,"total_pages":1}}`, accounts)
	}))
	t.Cleanup(srv.Close)
	apiClientFor(t, srv.URL)
}

func TestAccountsListAll_RendersTable(t *testing.T) {
	accountsAllServer(t, `[{"id":"a1","name":"Checking","account_type":"depository","currency":"EUR","balance":"€1.00","classification":"asset"}]`)
	format = "table"
	t.Cleanup(func() { format = "json" })
	cmd := newAccountsCmd()
	cmd.SetArgs([]string{"list", "--all"})
	out := captureStdout(t, func() { _ = cmd.Execute() })
	if strings.HasPrefix(strings.TrimSpace(out), "{") || !strings.Contains(out, "Checking") {
		t.Fatalf("--all --format table did not render a table:\n%s", out)
	}
}

func TestAccountsListAll_EmptyIsAnEmptyList(t *testing.T) {
	accountsAllServer(t, `[]`)
	cmd := newAccountsCmd()
	cmd.SetArgs([]string{"list", "--all"})
	out := captureStdout(t, func() { _ = cmd.Execute() })
	if !strings.Contains(out, `"accounts": []`) {
		t.Fatalf("empty --all must render accounts: [], got %s", out)
	}
}

func TestAccountsListAll_HasNoMetaStatus(t *testing.T) {
	// Output read from several pages is not one Sure response.
	accountsAllServer(t, `[{"id":"a1","name":"Checking","account_type":"depository","currency":"EUR","balance":"€1.00","classification":"asset"}]`)
	cmd := newAccountsCmd()
	cmd.SetArgs([]string{"list", "--all"})
	out := captureStdout(t, func() { _ = cmd.Execute() })
	if strings.Contains(out, `"status"`) || !strings.Contains(out, "Checking") {
		t.Fatalf("--all output must list accounts without meta.status, got %s", out)
	}
}
