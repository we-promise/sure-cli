package root

import (
	"errors"
	"fmt"
	"time"

	"github.com/dgilperez/sure-cli/internal/api"
	"github.com/dgilperez/sure-cli/internal/output"
	"github.com/spf13/cobra"
)

type txCreateOpts struct {
	AccountID string
	Date      string
	Amount    float64
	Nature    string
	Name      string
	Notes     string
	Currency  string
	Apply     bool
}

func newTransactionsCreateCmd() *cobra.Command {
	var o txCreateOpts

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a transaction (default dry-run; use --apply to execute)",
		Run: func(cmd *cobra.Command, args []string) {
			payload, err := buildTxCreatePayload(o)
			if err != nil {
				output.Fail("validation_failed", err.Error(), nil)
			}

			if !o.Apply {
				_ = output.Print(format, output.Envelope{Data: map[string]any{
					"dry_run": true,
					"request": map[string]any{
						"method": "POST",
						"path":   "/api/v1/transactions",
						"body":   payload,
					},
				}})
				return
			}

			client := api.New()
			var res any
			r, err := client.Post("/api/v1/transactions", payload, &res)
			if err != nil {
				output.Fail("request_failed", err.Error(), nil)
			}
			// Pass through status/body
			_ = output.Print(format, output.Envelope{Data: res, Meta: map[string]any{"status": r.StatusCode()}})
		},
	}

	cmd.Flags().StringVar(&o.AccountID, "account-id", "", "account id (required)")
	cmd.Flags().StringVar(&o.Date, "date", time.Now().Format("2006-01-02"), "date YYYY-MM-DD")
	cmd.Flags().Float64Var(&o.Amount, "amount", 0, "amount (absolute). Sign determined by --nature")
	cmd.Flags().StringVar(&o.Nature, "nature", "expense", "expense|income (maps to Sure transaction.nature)")
	cmd.Flags().StringVar(&o.Name, "name", "", "name/description (required)")
	cmd.Flags().StringVar(&o.Notes, "notes", "", "notes")
	cmd.Flags().StringVar(&o.Currency, "currency", "", "currency (default family currency)")
	cmd.Flags().BoolVar(&o.Apply, "apply", false, "execute the create (otherwise dry-run)")

	_ = cmd.MarkFlagRequired("account-id")
	_ = cmd.MarkFlagRequired("name")

	return cmd
}

func buildTxCreatePayload(o txCreateOpts) (map[string]any, error) {
	if o.AccountID == "" {
		return nil, errors.New("account-id is required")
	}
	if o.Name == "" {
		return nil, errors.New("name is required")
	}
	if o.Amount == 0 {
		return nil, errors.New("amount must be non-zero")
	}
	if _, err := time.Parse("2006-01-02", o.Date); err != nil {
		return nil, fmt.Errorf("invalid date (expected YYYY-MM-DD): %w", err)
	}
	switch o.Nature {
	case "income", "expense", "inflow", "outflow":
		// ok
	default:
		return nil, errors.New("nature must be one of: income|expense|inflow|outflow")
	}

	tx := map[string]any{
		"account_id": o.AccountID,
		"date":       o.Date,
		"amount":     fmt.Sprintf("%.2f", o.Amount),
		"nature":     o.Nature,
		"name":       o.Name,
		"notes":      o.Notes,
	}
	if o.Currency != "" {
		tx["currency"] = o.Currency
	}

	// Rails expects { transaction: {...} }
	return map[string]any{"transaction": tx}, nil
}
