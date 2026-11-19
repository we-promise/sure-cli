package root

import (
	"errors"
	"fmt"
	"net/url"
	"slices"
	"time"

	"github.com/spf13/cobra"
)

type cashFlowOpts struct {
	Month     string
	View      string
	GroupBy   string
	StartDate string
	EndDate   string
	Include   string
}

func newCashFlowCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "cash-flow", Short: "Income vs spending for a month (or a Sankey graph)"}

	var o cashFlowOpts
	show := &cobra.Command{
		Use:   "show",
		Short: "Show cash flow (defaults to the current month)",
		Long: `Show cash flow from GET /api/v1/cash_flow.

  --month YYYY-MM                    a past or current month (default: current month)
  --include sankey                   add the Sankey graph to the monthly summary
  --view sankey                      return only the Sankey graph (not with --include)
  --group-by category|account        Sankey grouping (requires --view sankey)
  --start-date/--end-date YYYY-MM-DD custom range (requires --view sankey, both dates, no --month)`,
		Args: cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			q, err := buildCashFlowQuery(o)
			failValidation(err)
			printGet(pathWithQuery("/api/v1/cash_flow", q))
		},
	}
	show.Flags().StringVar(&o.Month, "month", "", "month YYYY-MM (sent as YYYY-MM-01)")
	show.Flags().StringVar(&o.View, "view", "", "sankey: return only the graph")
	show.Flags().StringVar(&o.GroupBy, "group-by", "", "category|account (requires --view sankey)")
	show.Flags().StringVar(&o.StartDate, "start-date", "", "range start YYYY-MM-DD (requires --view sankey)")
	show.Flags().StringVar(&o.EndDate, "end-date", "", "range end YYYY-MM-DD (requires --view sankey)")
	show.Flags().StringVar(&o.Include, "include", "", "sankey: add the graph to the monthly summary")
	cmd.AddCommand(show)
	return cmd
}

// buildCashFlowQuery applies Api::V1::CashFlowsController's parameter rules
// client-side. Whether --month is in the future is left to the server, whose
// "today" is in the family's time zone.
func buildCashFlowQuery(o cashFlowOpts) (url.Values, error) {
	if o.Include != "" && o.View != "" {
		return nil, errors.New("--include and --view cannot be combined")
	}
	if !oneOfOrEmpty(o.Include, "sankey") || !oneOfOrEmpty(o.View, "sankey") {
		return nil, errors.New("--include and --view support only sankey")
	}
	if o.GroupBy != "" && o.View != "sankey" {
		return nil, errors.New("--group-by requires --view sankey")
	}
	if !oneOfOrEmpty(o.GroupBy, "category", "account") {
		return nil, errors.New("--group-by must be category or account")
	}

	q := url.Values{}
	hasRange, err := parsePairedDateRange(o.StartDate, o.EndDate)
	if err != nil {
		return nil, err
	}
	if hasRange {
		if o.View != "sankey" || o.Month != "" {
			return nil, errors.New("--start-date/--end-date require --view sankey and cannot be combined with --month")
		}
		q.Set("start_date", o.StartDate)
		q.Set("end_date", o.EndDate)
	} else if o.Month != "" {
		m, err := time.Parse("2006-01", o.Month)
		if err != nil {
			return nil, fmt.Errorf("invalid --month %q (expected YYYY-MM)", o.Month)
		}
		q.Set("month", m.Format(isoDateLayout))
	}

	for k, v := range map[string]string{"view": o.View, "group_by": o.GroupBy, "include": o.Include} {
		if v != "" {
			q.Set(k, v)
		}
	}
	return q, nil
}

func oneOfOrEmpty(v string, allowed ...string) bool {
	return v == "" || slices.Contains(allowed, v)
}
