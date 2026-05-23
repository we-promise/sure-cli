package root

import (
	"fmt"
	"net/url"

	"github.com/spf13/cobra"
	"github.com/we-promise/sure-cli/internal/api"
	"github.com/we-promise/sure-cli/internal/output"
)

func addInvestmentPagingFlags(cmd *cobra.Command, page, perPage *int) {
	cmd.Flags().IntVar(page, "page", 1, "page number")
	cmd.Flags().IntVar(perPage, "per-page", 25, "items per page (maps to per_page)")
}

func addInvestmentPagingQuery(q url.Values, page, perPage int) {
	if page > 0 {
		q.Set("page", fmt.Sprintf("%d", page))
	}
	if perPage > 0 {
		q.Set("per_page", fmt.Sprintf("%d", perPage))
	}
}

func addRepeatedInvestmentQuery(q url.Values, key string, values []string) {
	for _, v := range values {
		if v != "" {
			q.Add(key+"[]", v)
		}
	}
}

func investmentPathWithQuery(path string, q url.Values) string {
	if encoded := q.Encode(); encoded != "" {
		return path + "?" + encoded
	}
	return path
}

func printInvestmentGet(path string) {
	client := api.New()
	var res any
	r, err := client.Get(path, &res)
	if err != nil {
		output.Fail("request_failed", err.Error(), nil)
		return
	}
	if err := output.Print(format, output.Envelope{Data: res, Meta: &output.Meta{Status: r.StatusCode()}}); err != nil {
		output.Fail("output_failed", err.Error(), nil)
		return
	}
}

func printInvestmentPost(path string, body any) {
	client := api.New()
	var res any
	r, err := client.Post(path, body, &res)
	if err != nil {
		output.Fail("request_failed", err.Error(), nil)
		return
	}
	if err := output.Print(format, output.Envelope{Data: res, Meta: &output.Meta{Status: r.StatusCode()}}); err != nil {
		output.Fail("output_failed", err.Error(), nil)
		return
	}
}

func printInvestmentPatch(path string, body any) {
	client := api.New()
	var res any
	r, err := client.Patch(path, body, &res)
	if err != nil {
		output.Fail("request_failed", err.Error(), nil)
		return
	}
	if err := output.Print(format, output.Envelope{Data: res, Meta: &output.Meta{Status: r.StatusCode()}}); err != nil {
		output.Fail("output_failed", err.Error(), nil)
		return
	}
}

func printInvestmentDelete(path string) {
	client := api.New()
	var res any
	r, err := client.Delete(path, &res)
	if err != nil {
		output.Fail("request_failed", err.Error(), nil)
		return
	}
	if err := output.Print(format, output.Envelope{Data: res, Meta: &output.Meta{Status: r.StatusCode()}}); err != nil {
		output.Fail("output_failed", err.Error(), nil)
		return
	}
}

func printInvestmentDryRun(method, path string, body any) {
	request := map[string]any{
		"method": method,
		"path":   path,
	}
	if body != nil {
		request["body"] = body
	}
	if err := output.Print(format, output.Envelope{Data: map[string]any{
		"dry_run": true,
		"request": request,
	}}); err != nil {
		output.Fail("output_failed", err.Error(), nil)
		return
	}
}
