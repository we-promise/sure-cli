package root

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/we-promise/sure-cli/internal/models"
)

// rowsOf is a transactionSource over txs.
func rowsOf(txs []models.Transaction) transactionSource {
	return func(visit func(models.Transaction) error) error {
		for _, tx := range txs {
			if err := visit(tx); err != nil {
				return err
			}
		}
		return nil
	}
}

func TestExportTransactionsCSV_ExactAmountsAndReportingFlags(t *testing.T) {
	txs := []models.Transaction{
		{ID: "t1", Date: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC), Name: "Beans", Amount: -12.3456, AmountDecimal: "-12.3456", AmountText: "€12.35",
			Currency: "EUR", Classification: "expense", Kind: "standard", CategoryName: "Food", AccountName: "Main", MerchantName: "Roaster"},
		{ID: "t2", Date: time.Date(2026, 2, 2, 0, 0, 0, 0, time.UTC), Name: "Groceries", Amount: -100, AmountDecimal: "-100.0", AmountText: "€100.00",
			Currency: "EUR", Classification: "expense", Kind: "standard", AccountName: "Main", Excluded: true, SplitParent: true},
	}
	var buf bytes.Buffer
	if err := transactionWriters["csv"](&buf, rowsOf(txs)); err != nil {
		t.Fatalf("export: %v", err)
	}
	rows, err := csv.NewReader(&buf).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	want := [][]string{
		{"id", "date", "name", "amount", "currency", "classification", "kind", "category", "account", "merchant", "excluded", "split_parent"},
		{"t1", "2026-02-01", "Beans", "-12.3456", "EUR", "expense", "standard", "Food", "Main", "Roaster", "false", "false"},
		{"t2", "2026-02-02", "Groceries", "-100.0", "EUR", "expense", "standard", "", "Main", "", "true", "true"},
	}
	if !reflect.DeepEqual(rows, want) {
		t.Fatalf("csv rows\n got %v\nwant %v", rows, want)
	}
}

// exportFixture holds what an export must carry through unchanged: HTML
// characters, non-ASCII text, quotes and separators, zero values.
func exportFixture() []models.Transaction {
	return []models.Transaction{
		{ID: "t1", EntryID: "e1", Date: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC), Name: `Café "Le <Bon> & Co", Paris`, Amount: -12.3456, AmountDecimal: "-12.3456", AmountText: "€12.35",
			Currency: "EUR", Classification: "expense", Kind: "standard", CategoryName: "Food\nand drink", CategoryID: "c1", AccountID: "a1", AccountName: "Main", MerchantID: "m1", MerchantName: "Roaster", UserModified: true},
		{ID: "t2", Date: time.Date(2026, 2, 2, 0, 0, 0, 0, time.UTC), Name: "日本", Amount: 1000, AmountDecimal: "1000", AmountText: "¥1,000",
			Currency: "JPY", Classification: "income", Kind: "standard", AccountName: "Yen", Excluded: true, SplitParent: true},
		{ID: "t3"},
	}
}

// bufferedCSV and bufferedJSON are the export writers as they were when the
// export held every row in memory: the streamed files must match them byte
// for byte.
func bufferedCSV(out io.Writer, txs []models.Transaction) error {
	w := csv.NewWriter(out)
	_ = w.Write([]string{"id", "date", "name", "amount", "currency", "classification", "kind", "category", "account", "merchant", "excluded", "split_parent"})
	for _, tx := range txs {
		_ = w.Write([]string{tx.ID, tx.Date.Format(time.DateOnly), tx.Name, tx.AmountDecimal, tx.Currency, tx.Classification, tx.Kind,
			tx.CategoryName, tx.AccountName, tx.MerchantName, strconv.FormatBool(tx.Excluded), strconv.FormatBool(tx.SplitParent)})
	}
	w.Flush()
	return w.Error()
}

func bufferedJSON(out io.Writer, txs []models.Transaction, exportedAt string) error {
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(map[string]any{"exported_at": exportedAt, "count": len(txs), "transactions": txs})
}

func TestExportTransactions_StreamedFilesMatchTheBufferedOnes(t *testing.T) {
	for name, txs := range map[string][]models.Transaction{
		"rows": exportFixture(),
		"one":  exportFixture()[:1],
		// The buffered export held a nil slice when there were no rows.
		"none": nil,
	} {
		t.Run(name, func(t *testing.T) {
			var got, want bytes.Buffer
			if err := writeTransactionsCSV(&got, rowsOf(txs)); err != nil {
				t.Fatal(err)
			}
			_ = bufferedCSV(&want, txs)
			if got.String() != want.String() {
				t.Fatalf("csv\n got %q\nwant %q", got.String(), want.String())
			}

			got.Reset()
			want.Reset()
			if err := writeTransactionsJSON(&got, rowsOf(txs)); err != nil {
				t.Fatal(err)
			}
			var head struct {
				ExportedAt string `json:"exported_at"`
			}
			if err := json.Unmarshal(got.Bytes(), &head); err != nil {
				t.Fatalf("json export does not parse: %v\n%s", err, got.String())
			}
			_ = bufferedJSON(&want, txs, head.ExportedAt)
			if got.String() != want.String() {
				t.Fatalf("json\n got %s\nwant %s", got.String(), want.String())
			}
		})
	}
}

// failingWriter accepts nothing, like a full disk.
type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("no space left on device") }

func TestExportTransactions_ReportsWriteFailures(t *testing.T) {
	// The CSV writer buffers: a failed write only surfaces on Flush.
	txs := rowsOf([]models.Transaction{{ID: "t1", AmountDecimal: "-1"}})
	if err := writeTransactionsCSV(failingWriter{}, txs); err == nil {
		t.Error("CSV export to a failing writer reported success")
	}
	if err := writeTransactionsJSON(failingWriter{}, txs); err == nil {
		t.Error("JSON export to a failing writer reported success")
	}
}

func TestExportTransactions_ReportsSourceFailures(t *testing.T) {
	cut := errors.New("connection reset")
	source := func(visit func(models.Transaction) error) error {
		_ = visit(models.Transaction{ID: "t1"})
		return cut
	}
	for name, write := range transactionWriters {
		if err := write(io.Discard, source); !errors.Is(err, cut) {
			t.Errorf("%s: err = %v, want the source's error", name, err)
		}
	}
}

// exportTxServer serves pages of transactions; a page given as a status
// code fails with that status.
func exportTxServer(t *testing.T, pages ...any) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/v1/family_settings" {
			fmt.Fprint(w, `{"timezone":"UTC"}`)
			return
		}
		if r.URL.Path != "/api/v1/transactions" {
			t.Errorf("unexpected request %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		switch p := pages[page-1].(type) {
		case int:
			w.WriteHeader(p)
			fmt.Fprint(w, `{"error":"forbidden","message":"no"}`)
		case string:
			fmt.Fprintf(w, `{"transactions":[%s],"pagination":{"page":%d,"per_page":100,"total_pages":%d}}`, p, page, len(pages))
		}
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func TestExportTransactions_WritesEachRowOnceToAPrivateFile(t *testing.T) {
	// Offset paging served t2 twice: a sync added a row between the pages.
	url := exportTxServer(t,
		txJSON("t3", "2026-02-03", "c", "standard", "EUR", "3.0", false)+","+txJSON("t2", "2026-02-02", "b", "standard", "EUR", "2.0", false),
		txJSON("t2", "2026-02-02", "b", "standard", "EUR", "2.0", false)+","+txJSON("t1", "2026-02-01", "a", "standard", "EUR", "1.0", false),
	)
	apiClientFor(t, url)
	out := filepath.Join(t.TempDir(), "tx.json")
	cmd := newExportCmd()
	cmd.SetArgs([]string{"transactions", "--out", out, "--out-format", "json"})
	stdout := captureStdout(t, func() {
		format = "json"
		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(stdout, `"exported": 3`) {
		t.Fatalf("envelope = %s, want 3 rows exported", stdout)
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	var file struct {
		Count        int                  `json:"count"`
		Transactions []models.Transaction `json:"transactions"`
	}
	if err := json.Unmarshal(b, &file); err != nil {
		t.Fatal(err)
	}
	if file.Count != 3 || len(file.Transactions) != 3 || file.Transactions[2].ID != "t1" {
		t.Fatalf("file holds count %d and %d rows, want t3, t2, t1 once each", file.Count, len(file.Transactions))
	}
	if fi, _ := os.Stat(out); runtime.GOOS != "windows" && fi.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %v, want 0600", fi.Mode().Perm())
	}
}

func TestExportTransactions_AFailedFetchLeavesThePreviousFile(t *testing.T) {
	url := exportTxServer(t, txJSON("t1", "2026-02-01", "a", "standard", "EUR", "1.0", false), http.StatusForbidden)
	dir := t.TempDir()
	out := filepath.Join(dir, "tx.csv")
	if err := os.WriteFile(out, []byte("previous"), 0o600); err != nil {
		t.Fatal(err)
	}
	child, stdout := startCLI(t, "--config", apiKeyConfig(t, dir, url), "export", "transactions", "--out", out)
	if err := child.Wait(); err == nil {
		t.Fatalf("export succeeded although page 2 failed: %s", stdout)
	}
	if !strings.Contains(stdout.String(), `"code": "forbidden"`) {
		t.Fatalf("stdout = %s, want the page's forbidden error", stdout)
	}
	if b, _ := os.ReadFile(out); string(b) != "previous" {
		t.Fatalf("file = %q, want the previous export untouched", b)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 2 {
		t.Fatalf("dir holds %d entries, want tx.csv and config.yaml only", len(entries))
	}
}
