package root

import (
	"errors"
	"fmt"
	"time"

	"github.com/we-promise/sure-cli/internal/api"
	"github.com/we-promise/sure-cli/internal/output"
	"github.com/spf13/cobra"
)

type txUpdateOpts struct {
	ID         string
	Date       string
	Amount     float64
	Nature     string
	Name       string
	Notes      string
	Currency   string
	CategoryID string
	Apply      bool
}

func newTransactionsUpdateCmd() *cobra.Command {
	var o txUpdateOpts

	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update a transaction (default dry-run; use --apply to execute)",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			o.ID = args[0]
			payload, err := buildTxUpdatePayload(o)
			if err != nil {
				output.Fail("validation_failed", err.Error(), nil)
			}

			path := fmt.Sprintf("/api/v1/transactions/%s", o.ID)
			if !o.Apply {
				_ = output.Print(format, output.Envelope{Data: map[string]any{
					"dry_run": true,
					"request": map[string]any{
						"method": "PUT",
						"path":   path,
						"body":   payload,
					},
				}})
				return
			}

			client := api.New()
			var res any
			// resty supports Put via R().Put
			r, err := client.Put(path, payload, &res)
			if err != nil {
				output.Fail("request_failed", err.Error(), nil)
			}
			_ = output.Print(format, output.Envelope{Data: res, Meta: &output.Meta{Status: r.StatusCode()}})
		},
	}

	cmd.Flags().StringVar(&o.Date, "date", "", "date YYYY-MM-DD")
	cmd.Flags().Float64Var(&o.Amount, "amount", 0, "amount (absolute). Sign determined by --nature; omit to keep")
	cmd.Flags().StringVar(&o.Nature, "nature", "", "expense|income (required if --amount is set)")
	cmd.Flags().StringVar(&o.Name, "name", "", "name/description")
	cmd.Flags().StringVar(&o.Notes, "notes", "", "notes")
	cmd.Flags().StringVar(&o.Currency, "currency", "", "currency")
	cmd.Flags().StringVar(&o.CategoryID, "category-id", "", "category ID")
	cmd.Flags().BoolVar(&o.Apply, "apply", false, "execute the update (otherwise dry-run)")

	return cmd
}

func buildTxUpdatePayload(o txUpdateOpts) (map[string]any, error) {
	// require at least one field
	if o.Date == "" && o.Amount == 0 && o.Name == "" && o.Notes == "" && o.Currency == "" && o.CategoryID == "" {
		return nil, errors.New("no fields provided to update")
	}
	if o.Date != "" {
		if _, err := time.Parse("2006-01-02", o.Date); err != nil {
			return nil, fmt.Errorf("invalid date (expected YYYY-MM-DD): %w", err)
		}
	}
	if o.Amount != 0 {
		if o.Nature == "" {
			return nil, errors.New("nature is required when amount is provided")
		}
		switch o.Nature {
		case "income", "expense", "inflow", "outflow":
			// ok
		default:
			return nil, errors.New("nature must be one of: income|expense|inflow|outflow")
		}
	}

	tx := map[string]any{}
	if o.Date != "" {
		tx["date"] = o.Date
	}
	if o.Amount != 0 {
		tx["amount"] = fmt.Sprintf("%.2f", o.Amount)
		tx["nature"] = o.Nature
	}
	if o.Name != "" {
		tx["name"] = o.Name
	}
	if o.Notes != "" {
		tx["notes"] = o.Notes
	}
	if o.Currency != "" {
		tx["currency"] = o.Currency
	}
	if o.CategoryID != "" {
		tx["category_id"] = o.CategoryID
	}

	return map[string]any{"transaction": tx}, nil
}
