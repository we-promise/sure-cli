package root

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/we-promise/sure-cli/internal/api"
	errs "github.com/we-promise/sure-cli/internal/errors"
	"github.com/we-promise/sure-cli/internal/fsutil"
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

			if outFile == "" {
				outFile = fmt.Sprintf("transactions_%s.%s", end.Format(time.DateOnly), exportFormat)
			}

			// Every row, flagged: an export is a dump, not a total. Rows go
			// to the file page by page; a failed fetch leaves no file.
			exported := 0
			source := func(visit func(models.Transaction) error) error {
				return api.EachTransactionInWindow(client, start, end, func(tx models.Transaction) error {
					if err := visit(tx); err != nil {
						return exportFailed(err)
					}
					exported++
					return nil
				})
			}
			if err := fsutil.WriteFileAtomic(outFile, func(w io.Writer) error { return write(w, source) }); err != nil {
				var ce *errs.CLIError
				if !errors.As(err, &ce) {
					ce = exportFailed(err)
				}
				fail(ce)
			}

			printEnvelope(map[string]any{
				"exported":   exported,
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

func exportFailed(err error) *errs.CLIError {
	return errs.Wrap(errs.CodeExportFailed, err.Error(), err)
}

// transactionSource hands each transaction of an export to visit, in order,
// and stops at the first error.
type transactionSource func(visit func(models.Transaction) error) error

// transactionWriters maps each --out-format to the writer of its file.
var transactionWriters = map[string]func(io.Writer, transactionSource) error{
	"csv":  writeTransactionsCSV,
	"json": writeTransactionsJSON,
}

func writeTransactionsCSV(out io.Writer, each transactionSource) error {
	w := csv.NewWriter(out)

	// amount is exact and signed (income positive, expense negative).
	// excluded/split_parent rows are exported but do not count in totals.
	header := []string{"id", "date", "name", "amount", "currency", "classification", "kind", "category", "account", "merchant", "excluded", "split_parent"}
	if err := w.Write(header); err != nil {
		return err
	}
	err := each(func(tx models.Transaction) error {
		return w.Write([]string{
			csvText(tx.ID),
			tx.Date.Format(time.DateOnly),
			csvText(tx.Name),
			tx.AmountDecimal,
			csvText(tx.Currency),
			csvText(tx.Classification),
			csvText(tx.Kind),
			csvText(tx.CategoryName),
			csvText(tx.AccountName),
			csvText(tx.MerchantName),
			strconv.FormatBool(tx.Excluded),
			strconv.FormatBool(tx.SplitParent),
		})
	})
	if err != nil {
		return err
	}
	// csv.Writer buffers: a failed write only surfaces here.
	w.Flush()
	return w.Error()
}

// csvText keeps a text cell from running as a spreadsheet formula (CSV
// injection): a cell that starts with = + - @, a tab or a carriage return
// gets a leading apostrophe, which spreadsheets read as "this is text".
func csvText(s string) string {
	if s != "" && strings.ContainsRune("=+-@\t\r", rune(s[0])) {
		return "'" + s
	}
	return s
}

// writeTransactionsJSON writes {"count", "exported_at", "transactions"},
// indented by two spaces. count leads the object but is known only once every
// row is in, so the rows go to a temp file first and are copied out after.
func writeTransactionsJSON(out io.Writer, each transactionSource) (err error) {
	spool, err := os.CreateTemp("", "sure-cli-export-*.json")
	if err != nil {
		return err
	}
	defer func() {
		if cerr := spool.Close(); err == nil {
			err = cerr
		}
		_ = os.Remove(spool.Name())
	}()

	rows := bufio.NewWriter(spool)
	count := 0
	err = each(func(tx models.Transaction) error {
		row, err := json.MarshalIndent(tx, "    ", "  ")
		if err != nil {
			return err
		}
		sep := ",\n    "
		if count == 0 {
			sep = "\n    "
		}
		count++
		if _, err := rows.WriteString(sep); err != nil {
			return err
		}
		_, err = rows.Write(row)
		return err
	})
	if err != nil {
		return err
	}
	if err := rows.Flush(); err != nil {
		return err
	}
	if _, err := spool.Seek(0, io.SeekStart); err != nil {
		return err
	}

	exportedAt, _ := json.Marshal(time.Now().UTC().Format(time.RFC3339))
	if _, err := fmt.Fprintf(out, "{\n  \"count\": %d,\n  \"exported_at\": %s,\n  \"transactions\": ", count, exportedAt); err != nil {
		return err
	}
	if count == 0 {
		// No rows: null, as the export has always written them.
		_, err = io.WriteString(out, "null\n}\n")
		return err
	}
	if _, err := io.WriteString(out, "["); err != nil {
		return err
	}
	if _, err := io.Copy(out, spool); err != nil {
		return err
	}
	_, err = io.WriteString(out, "\n  ]\n}\n")
	return err
}
