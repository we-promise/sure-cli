package root

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/spf13/cobra"
	"github.com/we-promise/sure-cli/internal/api"
	"github.com/we-promise/sure-cli/internal/output"
)

func pathWithQuery(path string, q url.Values) string {
	if len(q) == 0 {
		return path
	}
	u := url.URL{Path: path, RawQuery: q.Encode()}
	return u.String()
}

func splitCSV(values []string) []string {
	var out []string
	for _, value := range values {
		for _, part := range strings.Split(value, ",") {
			part = strings.TrimSpace(part)
			if part != "" {
				out = append(out, part)
			}
		}
	}
	return out
}

func addRepeatedQuery(q url.Values, name string, values []string) {
	for _, value := range splitCSV(values) {
		q.Add(name+"[]", value)
	}
}

func addPagingFlags(cmd *cobra.Command, page, perPage *int) {
	cmd.Flags().IntVar(page, "page", 1, "page number")
	cmd.Flags().IntVar(perPage, "per-page", 25, "items per page (maps to per_page)")
}

func addPagingQuery(q url.Values, page, perPage int) {
	if page > 0 {
		q.Set("page", fmt.Sprintf("%d", page))
	}
	if perPage > 0 {
		q.Set("per_page", fmt.Sprintf("%d", perPage))
	}
}

func printGet(path string) {
	client := api.New()
	var res any
	r, err := client.Get(path, &res)
	if err != nil {
		output.Fail("request_failed", err.Error(), nil)
	}
	if err := output.Print(format, output.Envelope{Data: res, Meta: &output.Meta{Status: r.StatusCode()}}); err != nil {
		output.Fail("output_failed", err.Error(), nil)
	}
}

func printPost(path string, body any) {
	client := api.New()
	var res any
	r, err := client.Post(path, body, &res)
	if err != nil {
		output.Fail("request_failed", err.Error(), nil)
	}
	if err := output.Print(format, output.Envelope{Data: res, Meta: &output.Meta{Status: r.StatusCode()}}); err != nil {
		output.Fail("output_failed", err.Error(), nil)
	}
}

func printPatch(path string, body any) {
	client := api.New()
	var res any
	r, err := client.Patch(path, body, &res)
	if err != nil {
		output.Fail("request_failed", err.Error(), nil)
	}
	if err := output.Print(format, output.Envelope{Data: res, Meta: &output.Meta{Status: r.StatusCode()}}); err != nil {
		output.Fail("output_failed", err.Error(), nil)
	}
}

func printDelete(path string) {
	client := api.New()
	var res any
	r, err := client.Delete(path, &res)
	if err != nil {
		output.Fail("request_failed", err.Error(), nil)
	}
	if err := output.Print(format, output.Envelope{Data: res, Meta: &output.Meta{Status: r.StatusCode()}}); err != nil {
		output.Fail("output_failed", err.Error(), nil)
	}
}

func printDryRun(method, path string, body any) {
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
	}
}
