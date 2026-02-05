package root

import (
	"fmt"
	"net/url"

	"github.com/we-promise/sure-cli/internal/api"
	"github.com/we-promise/sure-cli/internal/output"
	"github.com/spf13/cobra"
)

func newAccountsCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "accounts", Short: "Accounts"}

	var page, perPage int
	list := &cobra.Command{
		Use:   "list",
		Short: "List accounts",
		Run: func(cmd *cobra.Command, args []string) {
			client := api.New()
			path := "/api/v1/accounts"
			if page > 0 || perPage > 0 {
				q := url.Values{}
				if page > 0 {
					q.Set("page", fmt.Sprintf("%d", page))
				}
				if perPage > 0 {
					q.Set("per_page", fmt.Sprintf("%d", perPage))
				}
				path = path + "?" + q.Encode()
			}
			var res any
			r, err := client.Get(path, &res)
			if err != nil {
				output.Fail("request_failed", err.Error(), nil)
			}
			_ = output.Print(format, output.Envelope{Data: res, Meta: &output.Meta{Status: r.StatusCode()}})
		},
	}
	list.Flags().IntVar(&page, "page", 1, "page number")
	list.Flags().IntVar(&perPage, "per-page", 25, "items per page (maps to per_page)")
	cmd.AddCommand(list)

	cmd.AddCommand(&cobra.Command{
		Use:   "show <id>",
		Short: "Show account (client-side lookup; API show is not implemented upstream yet)",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			// NOTE: Sure currently does not implement GET /api/v1/accounts/:id (route exists but controller/view missing).
			// Workaround: fetch list and find by id.
			client := api.New()
			var res map[string]any
			r, err := client.Get("/api/v1/accounts", &res)
			if err != nil {
				output.Fail("request_failed", err.Error(), nil)
			}
			accounts, _ := res["accounts"].([]any)
			for _, a := range accounts {
				m, ok := a.(map[string]any)
				if !ok {
					continue
				}
				if m["id"] == args[0] {
					_ = output.Print(format, output.Envelope{Data: m, Meta: &output.Meta{Status: r.StatusCode()}})
					return
				}
			}
			output.Fail("not_found", fmt.Sprintf("account %s not found", args[0]), map[string]any{"hint": "API endpoint GET /api/v1/accounts/:id is not implemented upstream; using list lookup"})
		},
	})

	return cmd
}
