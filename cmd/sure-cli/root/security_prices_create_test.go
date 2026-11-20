package root

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestReadSecurityPriceRows_Formats(t *testing.T) {
	want := []securityPriceRow{
		{Date: "2026-03-02", Price: "12.3400", Currency: "USD"},
		{Date: "2026-03-03", Price: "12.5", Currency: "USD"},
	}
	cases := map[string]string{
		"csv":              "date,price,currency\n2026-03-02,12.3400,USD\n2026-03-03,12.5,USD\n",
		"csv any order":    " Currency , DATE,price\r\nUSD, 2026-03-02 ,12.3400\r\nUSD,2026-03-03,12.5\r\n",
		"json array":       `[{"date":"2026-03-02","price":"12.3400","currency":"USD"},{"date":"2026-03-03","price":"12.5","currency":"USD"}]`,
		"json numbers":     `[{"date":"2026-03-02","price":12.3400,"currency":"USD"},{"date":"2026-03-03","price":12.5,"currency":"USD"}]`,
		"json body object": "\n  {\"prices\":[{\"date\":\"2026-03-02\",\"price\":\"12.3400\",\"currency\":\"USD\"},{\"date\":\"2026-03-03\",\"price\":12.5,\"currency\":\"USD\"}]}",
	}
	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := readSecurityPriceRows(strings.NewReader(input))
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("rows = %#v, want %#v", got, want)
			}
		})
	}
}

func TestReadSecurityPriceRows_RejectsMalformedInput(t *testing.T) {
	cases := map[string]string{
		"empty":                "  \n",
		"csv missing column":   "date,price\n2026-03-02,12\n",
		"csv unknown column":   "date,price,currency,note\n2026-03-02,12,USD,x\n",
		"csv duplicate column": "date,price,currency,price\n2026-03-02,12,USD,13\n",
		"csv ragged row":       "date,price,currency\n2026-03-02,12\n",
		"json unknown field":   `[{"date":"2026-03-02","price":"12","currency":"USD","provisional":true}]`,
		"json bool price":      `[{"date":"2026-03-02","price":true,"currency":"USD"}]`,
		"json trailing data":   `[{"date":"2026-03-02","price":"12","currency":"USD"}] []`,
		"json object no rows":  `{"security_id":"x"}`,
	}
	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			if rows, err := readSecurityPriceRows(strings.NewReader(input)); err == nil {
				t.Fatalf("expected error, got rows %#v", rows)
			}
		})
	}
}

func TestValidateSecurityPriceRows(t *testing.T) {
	row := func(date, price, currency string) securityPriceRow {
		return securityPriceRow{Date: date, Price: price, Currency: currency}
	}
	if err := validateSecurityPriceRows([]securityPriceRow{
		row("2026-03-02", "12.34000", "usd"),
		row("2026-03-03", "999999999999999.9999", "USD"),
		row("2024-02-29", "0.5", " USD "),
	}); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}

	cases := []struct {
		name    string
		rows    []securityPriceRow
		wantErr string
	}{
		{"no rows", nil, "prices must contain at least one row"},
		{"bad date format", []securityPriceRow{row("2026-3-2", "1", "USD")}, "prices[0]: date must be an ISO 8601 date"},
		{"impossible date", []securityPriceRow{row("2026-02-30", "1", "USD")}, "prices[0]: date must be an ISO 8601 date"},
		{"zero price", []securityPriceRow{row("2026-03-02", "0.00", "USD")}, "prices[0]: price must be greater than 0"},
		{"negative price", []securityPriceRow{row("2026-03-02", "-1", "USD")}, "prices[0]: price must be greater than 0"},
		{"non-numeric price", []securityPriceRow{row("2026-03-02", "12,5", "USD")}, "prices[0]: price must be a number"},
		{"trailing point", []securityPriceRow{row("2026-03-02", "3.", "USD")}, "prices[0]: price must be a number"},
		{"no leading digit", []securityPriceRow{row("2026-03-02", ".5", "USD")}, "prices[0]: price must be a number"},
		{"five decimals", []securityPriceRow{row("2026-03-02", "1.00001", "USD")}, "prices[0]: price must have at most 4 decimal places"},
		{"too large", []securityPriceRow{row("2026-03-02", "1000000000000000", "USD")}, "prices[0]: price is too large"},
		{"missing currency", []securityPriceRow{row("2026-03-02", "1", " ")}, "prices[0]: currency is required"},
		{"duplicate date", []securityPriceRow{row("2026-03-02", "1", "USD"), row("2026-03-02", "2", "USD")}, "prices[1]: date appears more than once in this batch"},
		{"mixed currency", []securityPriceRow{row("2026-03-02", "1", "USD"), row("2026-03-03", "2", "EUR")}, "prices[1]: every row in one request must use the same currency"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateSecurityPriceRows(tc.rows)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("err = %v, want containing %q", err, tc.wantErr)
			}
		})
	}

	t.Run("reports every bad row", func(t *testing.T) {
		err := validateSecurityPriceRows([]securityPriceRow{row("x", "1", "USD"), row("2026-03-03", "0", "USD")})
		rowErrs, ok := err.(securityPriceRowErrors)
		if !ok || len(rowErrs) != 2 || *rowErrs[0].Index != 0 || *rowErrs[1].Index != 1 {
			t.Fatalf("expected two indexed row errors, got %#v", err)
		}
	})

	t.Run("more than 1000 rows", func(t *testing.T) {
		rows := make([]securityPriceRow, securityPriceImportMaxRows+1)
		err := validateSecurityPriceRows(rows)
		if err == nil || !strings.Contains(err.Error(), "a request can carry at most 1000 prices") {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestSecurityPricesCreate_DryRunFromFileAndStdin(t *testing.T) {
	csv := "date,price,currency\n2026-03-02,12.3400,USD\n"
	path := filepath.Join(t.TempDir(), "prices.csv")
	if err := os.WriteFile(path, []byte(csv), 0o600); err != nil {
		t.Fatal(err)
	}
	wantBody := map[string]any{
		"security_id": "sec-1",
		"prices":      []any{map[string]any{"date": "2026-03-02", "price": "12.3400", "currency": "USD"}},
	}

	for name, file := range map[string]string{"file": path, "stdin": "-"} {
		t.Run(name, func(t *testing.T) {
			cmd := newSecurityPricesCmd()
			cmd.SetIn(strings.NewReader(csv))
			cmd.SetArgs([]string{"create", "--security-id", "sec-1", "--file", file})
			out := captureStdout(t, func() {
				format = "json"
				if err := cmd.Execute(); err != nil {
					t.Fatalf("execute: %v", err)
				}
			})
			var env struct {
				Data struct {
					DryRun  bool `json:"dry_run"`
					Request struct {
						Method string         `json:"method"`
						Path   string         `json:"path"`
						Body   map[string]any `json:"body"`
					} `json:"request"`
				} `json:"data"`
			}
			if err := json.Unmarshal([]byte(out), &env); err != nil {
				t.Fatalf("unmarshal: %v\nout=%q", err, out)
			}
			req := env.Data.Request
			if !env.Data.DryRun || req.Method != "POST" || req.Path != "/api/v1/security_prices" {
				t.Fatalf("unexpected dry run: %+v", env.Data)
			}
			if !reflect.DeepEqual(req.Body, wantBody) {
				t.Fatalf("body = %s, want %s", fmt.Sprint(req.Body), fmt.Sprint(wantBody))
			}
		})
	}
}
