package root

import (
	"fmt"
	"time"

	"github.com/we-promise/sure-cli/internal/api"
	"github.com/we-promise/sure-cli/internal/output"
	"github.com/we-promise/sure-cli/internal/rules"
	"github.com/spf13/cobra"
)

func newProposeCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "propose", Short: "Propose automations (rules)"}
	cmd.AddCommand(newProposeRulesCmd())
	return cmd
}

func newProposeRulesCmd() *cobra.Command {
	var months int
	var apply bool
	var minConfidence float64

	cmd := &cobra.Command{
		Use:   "rules",
		Short: "Propose categorization rules based on transaction patterns",
		Run: func(cmd *cobra.Command, args []string) {
			client := api.New()

			if months <= 0 {
				months = 3
			}
			end := time.Now().UTC()
			start := end.AddDate(0, -months, 0)
			txs, err := api.FetchTransactionsWindow(client, start, end, 500)
			if err != nil {
				output.Fail("request_failed", err.Error(), nil)
			}

			result := rules.ProposeRules(txs)

			if !apply {
				// Just show proposals
				_ = output.Print(format, output.Envelope{Data: result, Meta: &output.Meta{Schema: "docs/schemas/v1/propose_rules.schema.json", Status: 200}})
				return
			}

			// Apply mode: execute the proposals
			if minConfidence <= 0 {
				minConfidence = 0.8 // default safety threshold
			}

			var applied []map[string]any
			var skipped []map[string]any
			var errors []map[string]any

			for _, p := range result.Proposals {
				if p.Confidence < minConfidence {
					skipped = append(skipped, map[string]any{
						"pattern":    p.Pattern,
						"reason":     "confidence_below_threshold",
						"confidence": p.Confidence,
						"threshold":  minConfidence,
					})
					continue
				}
				if p.ValueID == "" {
					skipped = append(skipped, map[string]any{
						"pattern": p.Pattern,
						"reason":  "missing_category_id",
					})
					continue
				}

				// Apply to all affected transactions
				for _, txID := range p.AffectedTxIDs {
					path := fmt.Sprintf("/api/v1/transactions/%s", txID)
					payload := map[string]any{
						"transaction": map[string]any{
							"category_id": p.ValueID,
						},
					}
					var res any
					r, err := client.Put(path, payload, &res)
					if err != nil {
						errors = append(errors, map[string]any{
							"tx_id":   txID,
							"pattern": p.Pattern,
							"error":   err.Error(),
						})
						continue
					}
					if r.StatusCode() >= 400 {
						errors = append(errors, map[string]any{
							"tx_id":   txID,
							"pattern": p.Pattern,
							"error":   fmt.Sprintf("HTTP %d", r.StatusCode()),
						})
						continue
					}
					applied = append(applied, map[string]any{
						"tx_id":       txID,
						"pattern":     p.Pattern,
						"category":    p.Value,
						"category_id": p.ValueID,
					})
				}
			}

			_ = output.Print(format, output.Envelope{Data: map[string]any{
				"applied_count": len(applied),
				"skipped_count": len(skipped),
				"error_count":   len(errors),
				"applied":       applied,
				"skipped":       skipped,
				"errors":        errors,
			}, Meta: &output.Meta{Status: 200}})
		},
	}
	cmd.Flags().IntVar(&months, "months", 3, "lookback months")
	cmd.Flags().BoolVar(&apply, "apply", false, "execute the proposed rules (otherwise dry-run)")
	cmd.Flags().Float64Var(&minConfidence, "min-confidence", 0.8, "minimum confidence to apply (with --apply)")
	return cmd
}
