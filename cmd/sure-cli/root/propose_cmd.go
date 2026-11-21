package root

import (
	"fmt"
	"net/url"

	"github.com/spf13/cobra"

	"github.com/we-promise/sure-cli/internal/api"
	errs "github.com/we-promise/sure-cli/internal/errors"
	"github.com/we-promise/sure-cli/internal/output"
	"github.com/we-promise/sure-cli/internal/rules"
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
			// Confidence is a share of the affected rows: 0 applies everything,
			// above 1 nothing.
			if minConfidence <= 0 || minConfidence > 1 {
				failValidation(fmt.Errorf("--min-confidence must be above 0 and at most 1, got %v", minConfidence))
			}
			client := api.New()
			start, end, err := lookbackWindow("months", months, lookbackStart, familyClock(client))
			failValidation(err)
			txs, err := api.FetchTransactionsWindow(client, start, end)
			checkResponse(nil, err)

			result := rules.ProposeRules(txs)

			if !apply {
				// Just show proposals
				printEnvelope(result, &output.Meta{Schema: "docs/schemas/v1/propose_rules.schema.json"})
				return
			}

			report, failures := applyProposals(client, result.Proposals, minConfidence)
			if failures > 0 {
				fail(errs.New(errs.CodeApplyIncomplete, fmt.Sprintf("%d of the proposed updates failed", failures)).WithDetails(report))
			}
			printEnvelope(report, nil)
		},
	}
	cmd.Flags().IntVar(&months, "months", 3, "lookback months")
	cmd.Flags().BoolVar(&apply, "apply", false, "execute the proposed rules (otherwise dry-run)")
	cmd.Flags().Float64Var(&minConfidence, "min-confidence", 0.8, "minimum confidence to apply, above 0 and at most 1 (with --apply)")
	return cmd
}

// applyProposals sets each proposal's category on its affected transactions
// and reports what was applied, skipped and failed, and how many failed. A
// failed update does not
// stop the run, except a rate limit: every update after it would be refused
// too, so the rest are skipped as rate_limited.
func applyProposals(client *api.Client, proposals []rules.RuleProposal, minConfidence float64) (map[string]any, int) {
	applied := []map[string]any{}
	skipped := []map[string]any{}
	failed := []map[string]any{}
	rateLimited := false

	for _, p := range proposals {
		switch {
		case p.Confidence < minConfidence:
			skipped = append(skipped, map[string]any{
				"pattern":    p.Pattern,
				"reason":     "confidence_below_threshold",
				"confidence": p.Confidence,
				"threshold":  minConfidence,
			})
			continue
		case p.ValueID == "":
			skipped = append(skipped, map[string]any{
				"pattern": p.Pattern,
				"reason":  "missing_category_id",
			})
			continue
		}

		for _, txID := range p.AffectedTxIDs {
			if rateLimited {
				skipped = append(skipped, map[string]any{"tx_id": txID, "pattern": p.Pattern, "reason": errs.CodeRateLimit})
				continue
			}
			payload := map[string]any{"transaction": map[string]any{"category_id": p.ValueID}}
			var res any
			r, err := client.Put("/api/v1/transactions/"+url.PathEscape(txID), payload, &res)
			if ce := api.CheckResponse(r, err); ce != nil {
				failed = append(failed, map[string]any{
					"tx_id":   txID,
					"pattern": p.Pattern,
					"code":    ce.Code,
					"error":   ce.Message,
					"details": ce.Details,
				})
				rateLimited = ce.Code == errs.CodeRateLimit
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

	return map[string]any{
		"applied_count": len(applied),
		"skipped_count": len(skipped),
		"error_count":   len(failed),
		"applied":       applied,
		"skipped":       skipped,
		"errors":        failed,
	}, len(failed)
}
