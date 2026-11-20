package root

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"time"

	"github.com/spf13/cobra"
	"github.com/we-promise/sure-cli/internal/api"
	errs "github.com/we-promise/sure-cli/internal/errors"
	"github.com/we-promise/sure-cli/internal/models"
)

func newExportCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "export", Short: "Export data (transactions, accounts)"}
	cmd.AddCommand(newExportTransactionsCmd())
	return cmd
}

func newExportTransactionsCmd() *cobra.Command {
	var months int
	var outFile string
	// --out-format is the file's format; the persistent --format (json|table)
	// is how the result envelope prints.
	var exportFormat string

	cmd := &cobra.Command{
		Use:   "transactions",
		Short: "Export transactions to CSV or JSON",
		Run: func(cmd *cobra.Command, args []string) {
			write, ok := transactionWriters[exportFormat]
			if !ok {
				failValidation(errors.New("--out-format must be csv or json"))
			}
			start, end, err := lookbackWindow("months", months, lookbackStart, time.Now())
			failValidation(err)
			client := api.New()
			// Every row, flagged: an export is a dump, not a total.
			txs, err := api.FetchAllTransactionsWindow(client, start, end)
			checkResponse(nil, err)

			if outFile == "" {
				outFile = fmt.Sprintf("transactions_%s.%s", time.Now().Format(time.DateOnly), exportFormat)
			}

			if err := writeFile(outFile, func(w io.Writer) error { return write(w, txs) }); err != nil {
				fail(errs.Wrap(errs.CodeExportFailed, err.Error(), err))
			}

			printEnvelope(map[string]any{
				"exported":   len(txs),
				"file":       outFile,
				"out_format": exportFormat,
				"date_range": map[string]string{"start": start.Format(time.DateOnly), "end": end.Format(time.DateOnly)},
			}, nil)
		},
	}
	cmd.Flags().IntVar(&months, "months", 12, "lookback months")
	cmd.Flags().StringVar(&outFile, "out", "", "output file (default: transactions_DATE.FORMAT)")
	cmd.Flags().StringVar(&exportFormat, "out-format", "csv", "output file format (csv|json)")
	return cmd
}

// transactionWriters maps each --out-format to the writer of its file.
var transactionWriters = map[string]func(io.Writer, []models.Transaction) error{
	"csv":  writeTransactionsCSV,
	"json": writeTransactionsJSON,
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
			tx.Date.Format(time.DateOnly),
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
