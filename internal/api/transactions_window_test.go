package api

import (
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/we-promise/sure-cli/internal/config"
	errs "github.com/we-promise/sure-cli/internal/errors"
	"github.com/we-promise/sure-cli/internal/models"
)

// windowPages mirrors /api/v1/transactions as rendered by Sure's
// _transaction.json.jbuilder: a split parent (excluded, split_parent) with
// two children carrying its amount, an excluded row, a transfer leg, a row
// whose display string is rounded, a row from before amount_decimal (Sure
// #899 cents only), a legacy row with neither, and rows on an account
// excluded from reports and on a disabled account.
var windowPages = []string{
	`{"transactions":[
	  {"id":"t-exact","entry_id":"e-exact","date":"2026-02-01","amount":"€12.35","amount_decimal":"12.3456","amount_cents":1235,"signed_amount_cents":-1235,
	   "currency":"EUR","name":"Coffee beans","classification":"expense","kind":"standard","excluded":false,"split_parent":false,
	   "external_id":"bank-1","source":"api","user_modified":true,
	   "account":{"id":"a1","name":"Main"},"category":{"id":"c1","name":"Food"},"merchant":{"id":"m1","name":"Roaster"}},
	  {"id":"t-parent","entry_id":"e-parent","date":"2026-02-02","amount":"€100.00","amount_decimal":"100.0","currency":"EUR","name":"Groceries",
	   "classification":"expense","kind":"standard","excluded":true,"split_parent":true,"external_id":null,"source":null,
	   "account":{"id":"a1","name":"Main"},"category":null,"merchant":null},
	  {"id":"t-child-1","date":"2026-02-02","amount":"€60.00","amount_decimal":"60.0","currency":"EUR","name":"Groceries (food)",
	   "classification":"expense","kind":"standard","excluded":false,"split_parent":false,"account":{"id":"a1","name":"Main"}}
	],"pagination":{"page":1,"per_page":3,"total_pages":2}}`,
	`{"transactions":[
	  {"id":"t-child-2","date":"2026-02-02","amount":"€40.00","amount_decimal":"40.0","currency":"EUR","name":"Groceries (home)",
	   "classification":"expense","kind":"standard","excluded":false,"split_parent":false,"account":{"id":"a1","name":"Main"}},
	  {"id":"t-excluded","date":"2026-02-03","amount":"€7.00","amount_decimal":"7.0","currency":"EUR","name":"Refunded",
	   "classification":"expense","kind":"standard","excluded":true,"split_parent":false,"account":{"id":"a1","name":"Main"}},
	  {"id":"t-transfer","date":"2026-02-03","amount":"€500.00","amount_decimal":"500.0","currency":"EUR","name":"To savings",
	   "classification":"expense","kind":"funds_movement","excluded":false,"split_parent":false,"account":{"id":"a1","name":"Main"}},
	  {"id":"t-legacy","date":"2026-02-04","amount":"-€2,000.00","currency":"EUR","name":"Salary","classification":"income","account":{"id":"a1","name":"Main"}},
	  {"id":"t-cents","date":"2026-02-05","amount":"1.234,56 €","amount_cents":123456,"signed_amount_cents":-123456,"currency":"EUR","name":"Rent","classification":"expense","account":{"id":"a1","name":"Main"}},
	  {"id":"t-hidden","date":"2026-02-05","amount":"€9.00","amount_decimal":"9.0","currency":"EUR","name":"Hidden","classification":"expense","kind":"standard","account":{"id":"a-hidden","name":"Hidden"}},
	  {"id":"t-disabled","date":"2026-02-05","amount":"€8.00","amount_decimal":"8.0","currency":"EUR","name":"Old card","classification":"expense","kind":"standard","account":{"id":"a-disabled","name":"Old"}}
	],"pagination":{"page":2,"per_page":3,"total_pages":2}}`,
}

// windowAccounts is GET /api/v1/accounts without include_disabled: Sure
// lists draft and active accounts only, so a-disabled is absent.
const windowAccounts = `{"accounts":[
  {"id":"a1","name":"Main","currency":"EUR","status":"active","exclude_from_reports":false},
  {"id":"a-hidden","name":"Hidden","currency":"EUR","status":"active","exclude_from_reports":true}
],"pagination":{"page":1,"per_page":100,"total_pages":1}}`

func newWindowServer(t *testing.T, pages []string) {
	t.Helper()
	viper.Reset()
	_ = config.Init(t.TempDir() + "/config.yaml")
	viper.Set("auth.mode", "api_key")
	viper.Set("auth.api_key", "key")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/accounts":
			if r.URL.Query().Has("include_disabled") {
				t.Errorf("reportable accounts must come from the default (visible) list")
			}
			_, _ = w.Write([]byte(windowAccounts))
			return
		case "/api/v1/transactions":
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		if page < 1 || page > len(pages) {
			t.Errorf("unexpected page %d", page)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(pages[page-1]))
	}))
	t.Cleanup(srv.Close)
	viper.Set("api_url", srv.URL)
}

func window() (time.Time, time.Time) {
	return time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 2, 28, 0, 0, 0, 0, time.UTC)
}

func ids(txs []models.Transaction) []string {
	out := make([]string, len(txs))
	for i, tx := range txs {
		out[i] = tx.ID
	}
	return out
}

func TestFetchAllTransactionsWindow_DecodesEveryRow(t *testing.T) {
	newWindowServer(t, windowPages)
	start, end := window()
	txs, err := FetchAllTransactionsWindow(New(), start, end, 3)
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	want := []string{"t-exact", "t-parent", "t-child-1", "t-child-2", "t-excluded", "t-transfer", "t-legacy", "t-cents", "t-hidden", "t-disabled"}
	if got := ids(txs); !reflect.DeepEqual(got, want) {
		t.Fatalf("ids = %v, want %v", got, want)
	}

	wantExact := models.Transaction{
		ID: "t-exact", EntryID: "e-exact", Name: "Coffee beans", Classification: "expense", Kind: "standard",
		Amount: -12.3456, AmountDecimal: "-12.3456", AmountText: "€12.35", Currency: "EUR",
		Date: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC), AccountID: "a1", AccountName: "Main",
		MerchantID: "m1", MerchantName: "Roaster", CategoryName: "Food", CategoryID: "c1",
		ExternalID: "bank-1", Source: "api", UserModified: true,
	}
	if txs[0] != wantExact {
		t.Fatalf("decoded\n got %+v\nwant %+v", txs[0], wantExact)
	}

	parent := txs[1]
	if !parent.SplitParent || !parent.Excluded || parent.CategoryName != "" || parent.MerchantName != "" || parent.ExternalID != "" {
		t.Fatalf("split parent flags/nulls not decoded: %+v", parent)
	}

	legacy := txs[6]
	if legacy.Amount != 2000 || legacy.AmountDecimal != "2000.00" {
		t.Fatalf("legacy amount = (%v, %q), want (2000, \"2000.00\")", legacy.Amount, legacy.AmountDecimal)
	}
	cents := txs[7]
	if cents.Amount != -1234.56 || cents.AmountDecimal != "-1234.56" {
		t.Fatalf("cents amount = (%v, %q), want (-1234.56, \"-1234.56\")", cents.Amount, cents.AmountDecimal)
	}
}

func TestFetchTransactionsWindow_OnlyReportableRows(t *testing.T) {
	newWindowServer(t, windowPages)
	start, end := window()
	txs, err := FetchTransactionsWindow(New(), start, end, 3)
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	// Rows on an account excluded from reports, or not draft/active, are
	// dropped as Sure's income statement drops them.
	want := []string{"t-exact", "t-child-1", "t-child-2", "t-legacy", "t-cents"}
	if got := ids(txs); !reflect.DeepEqual(got, want) {
		t.Fatalf("ids = %v, want %v", got, want)
	}

	// The split's spend is counted once, through its children.
	var spend float64
	for _, tx := range txs {
		if tx.Classification == "expense" {
			spend -= tx.Amount
		}
	}
	if math.Abs(spend-1346.9056) > 1e-9 {
		t.Fatalf("expense total = %v, want 1346.9056", spend)
	}
}

func TestFetchTransactionsWindow_RejectsUnreadableAmount(t *testing.T) {
	newWindowServer(t, []string{`{"transactions":[{"id":"bad","date":"2026-02-01","amount":"","classification":"expense"}],"pagination":{"page":1,"total_pages":1}}`})
	start, end := window()
	if _, err := FetchTransactionsWindow(New(), start, end, 10); err == nil {
		t.Fatal("expected an error for a transaction whose amount cannot be read")
	}
}

func TestFetchTransactionsWindow_ReturnsTypedErrors(t *testing.T) {
	start, end := window()
	cases := []struct {
		name     string
		status   int
		body     string
		wantCode string
		wantAPI  string
	}{
		{"forbidden keeps Sure's code", http.StatusForbidden, `{"error":"insufficient_scope","message":"needs read"}`, errs.CodeAuthInvalid, "insufficient_scope"},
		{"server error", http.StatusInternalServerError, `{"error":"internal_server_error","message":"boom"}`, errs.CodeServerError, "internal_server_error"},
		{"malformed body", http.StatusOK, `{"transactions":"nope"}`, errs.CodeInvalidResponse, ""},
		{"unreadable amount", http.StatusOK, `{"transactions":[{"id":"bad","date":"2026-02-01","amount":"","classification":"expense"}]}`, errs.CodeInvalidResponse, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			viper.Reset()
			_ = config.Init(t.TempDir() + "/config.yaml")
			viper.Set("auth.mode", "api_key")
			viper.Set("auth.api_key", "key")
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			t.Cleanup(srv.Close)
			viper.Set("api_url", srv.URL)
			c := New()
			c.http.SetRetryCount(0)

			_, err := FetchTransactionsWindow(c, start, end, 10)
			var ce *errs.CLIError
			if !errors.As(err, &ce) {
				t.Fatalf("err = %v (%T), want *errs.CLIError", err, err)
			}
			if ce.Code != tc.wantCode {
				t.Fatalf("code = %q, want %q", ce.Code, tc.wantCode)
			}
			if tc.wantAPI != "" && ce.Details["api_error"] != tc.wantAPI {
				t.Fatalf("api_error = %v, want %q", ce.Details["api_error"], tc.wantAPI)
			}
		})
	}
}
