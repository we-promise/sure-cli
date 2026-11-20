package root

import (
	"fmt"
	"net/http"
	"net/url"
	"regexp"

	"github.com/go-resty/resty/v2"
	"github.com/spf13/cobra"

	"github.com/we-promise/sure-cli/internal/api"
	errs "github.com/we-promise/sure-cli/internal/errors"
	"github.com/we-promise/sure-cli/internal/output"
)

// enablePreviewFeatures tells the user how to pass Sure's preview-features
// gate on /api/v1/insights (Api::V1::InsightsController).
const enablePreviewFeatures = "turn on Preview features in Sure under Settings > Preferences for the user this CLI authenticates as, then retry"

func newInsightsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "insights",
		Short: "Sure's insights feed (requires preview features)",
		Long: "Insights Sure generates nightly: spending anomalies, cash flow warnings, " +
			"subscription audits, bank fees, budgets at risk and more.",
	}
	cmd.AddCommand(newInsightsListCmd())
	cmd.AddCommand(newInsightsAckCmd())
	return cmd
}

func newInsightsListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List the visible insights (active and read)",
		Long:  "Lists the insights Sure shows in its feed, active and read." + jsonOnly,
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			res, status, ce := fetchInsights()
			if ce != nil {
				output.Fail(ce.Code, ce.Message, ce.Details)
			}
			if err := output.Print(format, output.Envelope{Data: res, Meta: &output.Meta{
				Schema: "docs/schemas/v1/insights_list.schema.json",
				Status: status,
			}}); err != nil {
				output.Fail("output_failed", err.Error(), nil)
			}
		},
	}
}

func newInsightsAckCmd() *cobra.Command {
	var apply bool
	cmd := &cobra.Command{
		Use:   "ack <id>",
		Short: "Acknowledge an insight so it leaves the feed (default dry-run; use --apply to execute)",
		Long: "Acknowledges an insight so it leaves the feed. Sure answers the same way when " +
			"the insight was already acknowledged or had expired, so the result only says the " +
			"insight is no longer in the feed. Sure may show it again when its numbers change " +
			"materially or in a new period.",
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			failValidation(validateInsightID(args[0]))
			if !apply {
				printDryRun(http.MethodPatch, insightAckPath(args[0]), nil)
				return
			}
			status, ce := acknowledgeInsight(args[0])
			if ce != nil {
				output.Fail(ce.Code, ce.Message, ce.Details)
			}
			if err := output.Print(format, output.Envelope{
				Data: map[string]any{"id": args[0], "in_feed": false},
				Meta: &output.Meta{Status: status},
			}); err != nil {
				output.Fail("output_failed", err.Error(), nil)
			}
		},
	}
	cmd.Flags().BoolVar(&apply, "apply", false, "execute the request (otherwise dry-run)")
	return cmd
}

// uuidRE matches the canonical UUID text Sure's insight ids use.
var uuidRE = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// validateInsightID rejects ids Sure would answer with 404 (insight ids are
// UUIDs), so a typo fails before any request.
func validateInsightID(id string) error {
	if !uuidRE.MatchString(id) {
		return fmt.Errorf("insight id %q is not a UUID; take it from `insights list`", id)
	}
	return nil
}

func insightAckPath(id string) string {
	return "/api/v1/insights/" + url.PathEscape(id) + "/acknowledge"
}

// fetchInsights returns GET /api/v1/insights ({insights: [...]}, unpaginated)
// and its HTTP status, or a classified error.
func fetchInsights() (any, int, *errs.CLIError) {
	var res any
	r, err := api.New().Get("/api/v1/insights", &res)
	if ce := insightsError(r, err); ce != nil {
		return nil, 0, ce
	}
	return res, r.StatusCode(), nil
}

// acknowledgeInsight sends PATCH /api/v1/insights/{id}/acknowledge (204, no
// body) and returns its HTTP status, or a classified error.
func acknowledgeInsight(id string) (int, *errs.CLIError) {
	r, err := api.New().Patch(insightAckPath(id), nil, nil)
	if ce := insightsError(r, err); ce != nil {
		return 0, ce
	}
	return r.StatusCode(), nil
}

// insightsError is api.CheckResponse plus the remedy for Sure's 403
// feature_disabled, which these endpoints return until the user enables
// preview features.
func insightsError(r *resty.Response, err error) *errs.CLIError {
	ce := api.CheckResponse(r, err)
	if ce != nil && ce.Details["api_error"] == "feature_disabled" {
		ce.Message += ": " + enablePreviewFeatures
	}
	return ce
}
