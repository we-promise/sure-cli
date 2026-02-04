package root

import (
	"fmt"

	"github.com/dgilperez/sure-cli/internal/api"
	"github.com/dgilperez/sure-cli/internal/output"
	"github.com/spf13/cobra"
)

func newAccountsCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "accounts", Short: "Accounts"}

	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List accounts",
		Run: func(cmd *cobra.Command, args []string) {
			client := api.New()
			var res any
			r, err := client.Get("/api/v1/accounts", &res)
			if err != nil {
				output.Fail("request_failed", err.Error(), nil)
			}
			_ = output.PrintJSON(output.Envelope{Data: res, Meta: map[string]any{"status": r.StatusCode()}})
		},
	})

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
					_ = output.PrintJSON(output.Envelope{Data: m, Meta: map[string]any{"status": r.StatusCode(), "source": "list"}})
					return
				}
			}
			output.Fail("not_found", fmt.Sprintf("account %s not found", args[0]), map[string]any{"hint": "API endpoint GET /api/v1/accounts/:id is not implemented upstream; using list lookup"})
		},
	})

	return cmd
}
