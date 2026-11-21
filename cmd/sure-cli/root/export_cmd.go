package root

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/we-promise/sure-cli/internal/api"
	errs "github.com/we-promise/sure-cli/internal/errors"
	"github.com/we-promise/sure-cli/internal/fsutil"
	"github.com/we-promise/sure-cli/internal/models"
	"github.com/we-promise/sure-cli/internal/output"
)

func newExportCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "export", Short: "Export transactions"}
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
			client := api.New()
			start, end, err := lookbackWindow("months", months, lookbackStart, familyClock(client))
			failValidation(err)

			if outFile == "" {
				outFile = fmt.Sprintf("transactions_%s.%s", end.Format(time.DateOnly), exportFormat)
			}
			if fsutil.IsStdout(outFile) {
				output.EnvelopesToStderr() // stdout carries the file
			}

			// The JSON spool goes next to the file, on its disk and under its
			// permissions; a pipe, a device or stdout (/dev/stdout redirected
			// to a file is not a stream, and /dev is not writable) has no
			// directory to share.
			spoolDir := filepath.Dir(outFile)
			if fsutil.IsStream(outFile) || fsutil.IsStdout(outFile) {
				spoolDir = ""
			}
			ctx, stop := interruptContext()
			defer stop()

			// Every row, flagged: an export is a dump, not a total. Rows go
			// to the file page by page; a failed fetch leaves no file.
			exported := 0
			source := func(visit func(models.Transaction) error) error {
				return api.EachTransactionInWindow(ctx, client, start, end, api.StreamMaxPages, func(tx models.Transaction) error {
					if err := visit(tx); err != nil {
						return exportFailed(err)
					}
					exported++
					return nil
				})
			}
			if err := fsutil.WriteFileAtomic(outFile, func(w io.Writer) error { return write(w, spoolDir, source) }); err != nil {
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

// transactionWriters maps each --out-format to the writer of its file. A
// writer that needs scratch space puts it in spoolDir ("" for the system's
// temp directory).
var transactionWriters = map[string]func(out io.Writer, spoolDir string, each transactionSource) error{
	"csv":  writeTransactionsCSV,
	"json": writeTransactionsJSON,
}

// exportRow is one exported transaction: the CSV columns, in order, and the
// keys of a JSON row. amount is the exact signed decimal (income positive,
// expense negative); excluded and split_parent rows are exported but do not
// count in totals.
type exportRow struct {
	ID             string `json:"id"`
	Date           string `json:"date"`
	Name           string `json:"name"`
	Amount         string `json:"amount"`
	Currency       string `json:"currency"`
	Classification string `json:"classification"`
	Kind           string `json:"kind"`
	Category       string `json:"category"`
	Account        string `json:"account"`
	Merchant       string `json:"merchant"`
	Excluded       bool   `json:"excluded"`
	SplitParent    bool   `json:"split_parent"`
}

var exportColumns = []string{"id", "date", "name", "amount", "currency", "classification", "kind", "category", "account", "merchant", "excluded", "split_parent"}

func newExportRow(tx models.Transaction) exportRow {
	return exportRow{
		ID: tx.ID, Date: tx.Date.Format(time.DateOnly), Name: tx.Name, Amount: tx.AmountDecimal,
		Currency: tx.Currency, Classification: tx.Classification, Kind: tx.Kind,
		Category: tx.CategoryName, Account: tx.AccountName, Merchant: tx.MerchantName,
		Excluded: tx.Excluded, SplitParent: tx.SplitParent,
	}
}

// csvRecord is r in exportColumns order, text cells escaped by csvText.
func (r exportRow) csvRecord() []string {
	return []string{
		csvText(r.ID), r.Date, csvText(r.Name), r.Amount, csvText(r.Currency), csvText(r.Classification),
		csvText(r.Kind), csvText(r.Category), csvText(r.Account), csvText(r.Merchant),
		strconv.FormatBool(r.Excluded), strconv.FormatBool(r.SplitParent),
	}
}

func writeTransactionsCSV(out io.Writer, _ string, each transactionSource) error {
	w := csv.NewWriter(out)
	if err := w.Write(exportColumns); err != nil {
		return err
	}
	err := each(func(tx models.Transaction) error {
		return w.Write(newExportRow(tx).csvRecord())
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
// each transaction an exportRow, indented by two spaces. count leads the object but is known only once every
// row is in, so the rows go to a temp file in spoolDir first and are copied
// out after.
func writeTransactionsJSON(out io.Writer, spoolDir string, each transactionSource) (err error) {
	spool, err := os.CreateTemp(spoolDir, ".sure-cli-export-*.spool")
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
		row, err := json.MarshalIndent(newExportRow(tx), "    ", "  ")
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
		_, err = io.WriteString(out, "[]\n}\n")
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
