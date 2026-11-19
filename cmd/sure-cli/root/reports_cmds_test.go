package root

import (
	"net/url"
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
	cases := map[string]cashFlowOpts{
		"include and view combined":    {Include: "sankey", View: "sankey"},
		"unknown view":                 {View: "table"},
		"unknown include":              {Include: "daily"},
		"group-by without sankey view": {GroupBy: "category"},
		"group-by with include only":   {Include: "sankey", GroupBy: "account"},
		"unknown group-by":             {View: "sankey", GroupBy: "merchant"},
		"range without sankey view":    {StartDate: "2026-01-01", EndDate: "2026-01-31"},
		"range with month":             {View: "sankey", Month: "2026-01", StartDate: "2026-01-01", EndDate: "2026-01-31"},
		"start date only":              {View: "sankey", StartDate: "2026-01-01"},
		"end date only":                {View: "sankey", EndDate: "2026-01-31"},
		"range out of order":           {View: "sankey", StartDate: "2026-02-01", EndDate: "2026-01-31"},
		"invalid start date":           {View: "sankey", StartDate: "2026-02-30", EndDate: "2026-03-31"},
		"month with day":               {Month: "2026-03-01"},
		"month not a month":            {Month: "2026-13"},
		"month in another format":      {Month: "03/2026"},
	}
	for name, o := range cases {
		if _, err := buildCashFlowQuery(o); err == nil {
			t.Fatalf("%s: expected error", name)
		}
	}
}
