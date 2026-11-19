package root

import (
	"fmt"
	"net/url"
	"time"

	"github.com/spf13/cobra"
	"github.com/we-promise/sure-cli/internal/api"
	"github.com/we-promise/sure-cli/internal/output"
	"github.com/we-promise/sure-cli/internal/plan"
)

func newPlanCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "plan", Short: "Planning commands (runway)"}
	cmd.AddCommand(newPlanRunwayCmd())
	return cmd
}

func newPlanRunwayCmd() *cobra.Command {
	var accountID string
	var windowDays int
	cmd := &cobra.Command{
		Use:   "runway",
		Short: "Estimate runway months based on recent spending",
		Run: func(cmd *cobra.Command, args []string) {
			if accountID == "" {
				output.Fail("missing_account", "--account-id is required", nil)
				return
			}
			client := api.New()

			// Fetch the account directly; include_disabled so a disabled
			// account can still be planned against when asked for by id.
			var acc map[string]any
			path := pathWithQuery("/api/v1/accounts/"+url.PathEscape(accountID), api.AccountsQuery(true))
			r, err := client.Get(path, &acc)
			checkResponse(r, err)
			bal, ok := amountFromAPIOK(acc, "balance", "balance_cents")
			if !ok {
				output.Fail("account_balance_missing", "account has no usable balance", map[string]any{"account_id": accountID})
				return
			}

			if windowDays <= 0 {
				windowDays = 90
			}

			end := time.Now().UTC()
			start := end.AddDate(0, 0, -windowDays)
			txs, err := api.FetchTransactionsWindow(client, start, end, 200)
			if err != nil {
				output.Fail("request_failed", err.Error(), nil)
				return
			}

			out := plan.ComputeRunway(bal, fmt.Sprint(acc["currency"]), txs, windowDays)
			_ = output.Print(format, output.Envelope{Data: out, Meta: &output.Meta{Schema: "docs/schemas/v1/plan_runway.schema.json", Status: 200}})
		},
	}
	cmd.Flags().StringVar(&accountID, "account-id", "", "cash account id")
	cmd.Flags().IntVar(&windowDays, "days", 90, "lookback days")
	return cmd
}
