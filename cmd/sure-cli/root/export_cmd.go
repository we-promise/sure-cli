package root

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"time"

	"github.com/spf13/cobra"
	"github.com/we-promise/sure-cli/internal/api"
	"github.com/we-promise/sure-cli/internal/models"
	"github.com/we-promise/sure-cli/internal/output"
)

func newExportCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "export", Short: "Export data (transactions, accounts)"}
	cmd.AddCommand(newExportTransactionsCmd())
	return cmd
}

func newExportTransactionsCmd() *cobra.Command {
	var months int
	var outFile string
	// Local --out-format avoids colliding with the persistent --format
	// (json|table) declared on the root command. Previously both flags were
	// named --format and only worked because they happened to be read from
	// different package-level variables.
	var exportFormat string

	cmd := &cobra.Command{
		Use:   "transactions",
		Short: "Export transactions to CSV or JSON",
		Run: func(cmd *cobra.Command, args []string) {
			client := api.New()

			if months <= 0 {
				months = 12
			}
			end := time.Now().UTC()
			start := lookbackStart(end, months)
			// Every row, flagged: an export is a dump, not a total.
			txs, err := api.FetchAllTransactionsWindow(client, start, end, 1000)
			checkResponse(nil, err)

			if outFile == "" {
				outFile = fmt.Sprintf("transactions_%s.%s", time.Now().Format("2006-01-02"), exportFormat)
			}

			switch exportFormat {
			case "csv":
				if err := exportTransactionsCSV(txs, outFile); err != nil {
					output.Fail("export_failed", err.Error(), nil)
					return
				}
			case "json":
				if err := exportTransactionsJSON(txs, outFile); err != nil {
					output.Fail("export_failed", err.Error(), nil)
					return
				}
			default:
				output.Fail("invalid_format", "out-format must be csv or json", nil)
				return
			}

			_ = output.Print(format, output.Envelope{Data: map[string]any{
				"exported":   len(txs),
				"file":       outFile,
				"out_format": exportFormat,
				"date_range": map[string]string{"start": start.Format("2006-01-02"), "end": end.Format("2006-01-02")},
			}, Meta: &output.Meta{Status: 200}})
		},
	}
	cmd.Flags().IntVar(&months, "months", 12, "lookback months")
	cmd.Flags().StringVar(&outFile, "out", "", "output file (default: transactions_DATE.FORMAT)")
	cmd.Flags().StringVar(&exportFormat, "out-format", "csv", "output file format (csv|json)")
	return cmd
}

func exportTransactionsCSV(txs []models.Transaction, path string) error {
	return writeFile(path, func(w io.Writer) error { return writeTransactionsCSV(w, txs) })
}

func exportTransactionsJSON(txs []models.Transaction, path string) error {
	return writeFile(path, func(w io.Writer) error { return writeTransactionsJSON(w, txs) })
}

// writeFile creates path and hands it to write. A failure to write or to
// close the file is the error: the data may not be on disk.
func writeFile(path string, write func(io.Writer) error) (err error) {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := f.Close(); err == nil {
			err = cerr
		}
	}()
	return write(f)
}

func writeTransactionsCSV(out io.Writer, txs []models.Transaction) error {
	w := csv.NewWriter(out)

	// amount is exact and signed (income positive, expense negative).
	// excluded/split_parent rows are exported but do not count in totals.
	header := []string{"id", "date", "name", "amount", "currency", "classification", "kind", "category", "account", "merchant", "excluded", "split_parent"}
	if err := w.Write(header); err != nil {
		return err
	}
	for _, tx := range txs {
		row := []string{
			tx.ID,
			tx.Date.Format("2006-01-02"),
			tx.Name,
			tx.AmountDecimal,
			tx.Currency,
			tx.Classification,
			tx.Kind,
			tx.CategoryName,
			tx.AccountName,
			tx.MerchantName,
			strconv.FormatBool(tx.Excluded),
			strconv.FormatBool(tx.SplitParent),
		}
		if err := w.Write(row); err != nil {
			return err
		}
	}
	// csv.Writer buffers: a failed write only surfaces here.
	w.Flush()
	return w.Error()
}

func writeTransactionsJSON(out io.Writer, txs []models.Transaction) error {
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(map[string]any{
		"exported_at":  time.Now().UTC().Format(time.RFC3339),
		"count":        len(txs),
		"transactions": txs,
	})
}
