package root

import (
	"net/url"
	"strings"
	"testing"
)

func assertQuery(t *testing.T, q url.Values, want map[string]string) {
	t.Helper()
	if len(q) != len(want) {
		t.Fatalf("query = %v, want %v", q, want)
	}
	for k, v := range want {
		if got := q.Get(k); got != v {
			t.Fatalf("%s = %q, want %q (query %v)", k, got, v, q)
		}
	}
}

func TestReportCommandsRegistered(t *testing.T) {
	root := New()
	for _, path := range [][]string{
		{"cash-flow"},
		{"cash-flow", "show"},
		{"spending-vs-normal"},
		{"spending-vs-normal", "show"},
	} {
		got, _, err := root.Find(path)
		if err != nil {
			t.Fatalf("path %v not registered: %v", path, err)
		}
		if got.Name() != path[len(path)-1] {
			t.Fatalf("path %v resolved to %q", path, got.Name())
		}
	}
}

func TestCashFlowShowFlags(t *testing.T) {
	show := findSub(t, newCashFlowCmd(), "show")
	for _, name := range []string{"month", "view", "group-by", "start-date", "end-date", "include"} {
		if show.Flags().Lookup(name) == nil {
			t.Fatalf("expected flag %q on cash-flow show", name)
		}
	}
	if show.Args == nil {
		t.Fatal("cash-flow show should reject positional args")
	}
}

func TestBuildCashFlowQuery_Valid(t *testing.T) {
	cases := map[string]struct {
		in   cashFlowOpts
		want map[string]string
	}{
		"current month (no params)": {cashFlowOpts{}, map[string]string{}},
		"month converted to first day": {
			cashFlowOpts{Month: "2026-03"},
			map[string]string{"month": "2026-03-01"},
		},
		"month with sankey included": {
			cashFlowOpts{Month: "2026-03", Include: "sankey"},
			map[string]string{"month": "2026-03-01", "include": "sankey"},
		},
		"sankey view grouped by account for a month": {
			cashFlowOpts{Month: "2026-03", View: "sankey", GroupBy: "account"},
			map[string]string{"month": "2026-03-01", "view": "sankey", "group_by": "account"},
		},
		"sankey view over a custom range": {
			cashFlowOpts{View: "sankey", StartDate: "2026-01-01", EndDate: "2026-03-31", GroupBy: "category"},
			map[string]string{"view": "sankey", "start_date": "2026-01-01", "end_date": "2026-03-31", "group_by": "category"},
		},
		"single-day range": {
			cashFlowOpts{View: "sankey", StartDate: "2026-01-01", EndDate: "2026-01-01"},
			map[string]string{"view": "sankey", "start_date": "2026-01-01", "end_date": "2026-01-01"},
		},
	}
	for name, c := range cases {
		q, err := buildCashFlowQuery(c.in)
		if err != nil {
			t.Fatalf("%s: unexpected error: %v", name, err)
		}
		assertQuery(t, q, c.want)
	}
}

func TestBuildCashFlowQuery_Rejections(t *testing.T) {
	// Each case names the rule that must reject it, in
	// Api::V1::CashFlowsController's order.
	cases := map[string]struct {
		opts cashFlowOpts
		want string
	}{
		"include and view combined":    {cashFlowOpts{Include: "sankey", View: "sankey"}, "--include and --view cannot be combined"},
		"unknown view":                 {cashFlowOpts{View: "table"}, "support only sankey"},
		"unknown include":              {cashFlowOpts{Include: "daily"}, "support only sankey"},
		"group-by without sankey view": {cashFlowOpts{GroupBy: "category"}, "--group-by requires --view sankey"},
		"group-by with include only":   {cashFlowOpts{Include: "sankey", GroupBy: "account"}, "--group-by requires --view sankey"},
		"unknown group-by":             {cashFlowOpts{View: "sankey", GroupBy: "merchant"}, "--group-by must be category or account"},
		"range without sankey view":    {cashFlowOpts{StartDate: "2026-01-01", EndDate: "2026-01-31"}, "require --view sankey"},
		"range with month":             {cashFlowOpts{View: "sankey", Month: "2026-01", StartDate: "2026-01-01", EndDate: "2026-01-31"}, "cannot be combined with --month"},
		"start date only":              {cashFlowOpts{View: "sankey", StartDate: "2026-01-01"}, "--start-date and --end-date must be given together"},
		"end date only":                {cashFlowOpts{View: "sankey", EndDate: "2026-01-31"}, "--start-date and --end-date must be given together"},
		"range out of order":           {cashFlowOpts{View: "sankey", StartDate: "2026-02-01", EndDate: "2026-01-31"}, "--start-date must be on or before --end-date"},
		"invalid start date":           {cashFlowOpts{View: "sankey", StartDate: "2026-02-30", EndDate: "2026-03-31"}, "--start-date: invalid date"},
		"month with day":               {cashFlowOpts{Month: "2026-03-01"}, "invalid --month"},
		"month not a month":            {cashFlowOpts{Month: "2026-13"}, "invalid --month"},
		"month in another format":      {cashFlowOpts{Month: "03/2026"}, "invalid --month"},
	}
	for name, c := range cases {
		if _, err := buildCashFlowQuery(c.opts); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: err = %v, want %q", name, err, c.want)
		}
	}
}

func TestSpendingVsNormalShowFlags(t *testing.T) {
	show := findSub(t, newSpendingVsNormalCmd(), "show")
	for _, name := range []string{"start-date", "end-date"} {
		if show.Flags().Lookup(name) == nil {
			t.Fatalf("expected flag %q on spending-vs-normal show", name)
		}
	}
	if show.Args == nil {
		t.Fatal("spending-vs-normal show should reject positional args")
	}
}

func TestBuildSpendingVsNormalQuery(t *testing.T) {
	q, err := buildSpendingVsNormalQuery("", "")
	if err != nil {
		t.Fatalf("neither date: unexpected error: %v", err)
	}
	assertQuery(t, q, map[string]string{})

	q, err = buildSpendingVsNormalQuery("2026-01-01", "2026-03-31")
	if err != nil {
		t.Fatalf("both dates: unexpected error: %v", err)
	}
	assertQuery(t, q, map[string]string{"start_date": "2026-01-01", "end_date": "2026-03-31"})

	for name, in := range map[string][2]string{
		"start only":   {"2026-01-01", ""},
		"end only":     {"", "2026-01-31"},
		"out of order": {"2026-02-01", "2026-01-31"},
		"invalid date": {"2026-01-01", "2026-02-30"},
	} {
		if _, err := buildSpendingVsNormalQuery(in[0], in[1]); err == nil {
			t.Fatalf("%s: expected error", name)
		}
	}
}
