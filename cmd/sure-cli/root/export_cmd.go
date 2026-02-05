package root

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/we-promise/sure-cli/internal/api"
	"github.com/we-promise/sure-cli/internal/models"
	"github.com/we-promise/sure-cli/internal/output"
	"github.com/spf13/cobra"
)

func newExportCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "export", Short: "Export data (transactions, accounts)"}
	cmd.AddCommand(newExportTransactionsCmd())
	return cmd
}

func newExportTransactionsCmd() *cobra.Command {
	var months int
	var outFile string
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
			start := end.AddDate(0, -months, 0)
			txs, err := api.FetchTransactionsWindow(client, start, end, 1000)
			if err != nil {
				output.Fail("request_failed", err.Error(), nil)
			}

			if outFile == "" {
				outFile = fmt.Sprintf("transactions_%s.%s", time.Now().Format("2006-01-02"), exportFormat)
			}

			switch exportFormat {
			case "csv":
				if err := exportTransactionsCSV(txs, outFile); err != nil {
					output.Fail("export_failed", err.Error(), nil)
				}
			case "json":
				if err := exportTransactionsJSON(txs, outFile); err != nil {
					output.Fail("export_failed", err.Error(), nil)
				}
			default:
				output.Fail("invalid_format", "format must be csv or json", nil)
			}

			_ = output.Print(format, output.Envelope{Data: map[string]any{
				"exported":   len(txs),
				"file":       outFile,
				"format":     exportFormat,
				"date_range": map[string]string{"start": start.Format("2006-01-02"), "end": end.Format("2006-01-02")},
			}, Meta: &output.Meta{Status: 200}})
		},
	}
	cmd.Flags().IntVar(&months, "months", 12, "lookback months")
	cmd.Flags().StringVar(&outFile, "out", "", "output file (default: transactions_DATE.FORMAT)")
	cmd.Flags().StringVar(&exportFormat, "format", "csv", "export format (csv|json)")
	return cmd
}

func exportTransactionsCSV(txs []models.Transaction, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	// Header
	header := []string{"id", "date", "name", "amount", "currency", "classification", "category", "account", "merchant"}
	if err := w.Write(header); err != nil {
		return err
	}

	// Rows
	for _, tx := range txs {
		row := []string{
			tx.ID,
			tx.Date.Format("2006-01-02"),
			tx.Name,
			tx.AmountText,
			tx.Currency,
			tx.Classification,
			tx.CategoryName,
			tx.AccountName,
			tx.MerchantName,
		}
		if err := w.Write(row); err != nil {
			return err
		}
	}

	return nil
}

func exportTransactionsJSON(txs []models.Transaction, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")

	data := map[string]any{
		"exported_at":  time.Now().UTC().Format(time.RFC3339),
		"count":        len(txs),
		"transactions": txs,
	}
	return enc.Encode(data)
}
