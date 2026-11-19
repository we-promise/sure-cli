package root

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/we-promise/sure-cli/internal/models"
)

func TestExportTransactionsCSV_ExactAmountsAndReportingFlags(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tx.csv")
	txs := []models.Transaction{
		{ID: "t1", Date: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC), Name: "Beans", Amount: -12.3456, AmountDecimal: "-12.3456", AmountText: "€12.35",
			Currency: "EUR", Classification: "expense", Kind: "standard", CategoryName: "Food", AccountName: "Main", MerchantName: "Roaster"},
		{ID: "t2", Date: time.Date(2026, 2, 2, 0, 0, 0, 0, time.UTC), Name: "Groceries", Amount: -100, AmountDecimal: "-100.0", AmountText: "€100.00",
			Currency: "EUR", Classification: "expense", Kind: "standard", AccountName: "Main", Excluded: true, SplitParent: true},
	}
	if err := exportTransactionsCSV(txs, path); err != nil {
		t.Fatalf("export: %v", err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	rows, err := csv.NewReader(f).ReadAll()
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
