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
		Short: "Show account",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			client := api.New()
			var res any
			path := fmt.Sprintf("/api/v1/accounts/%s", args[0])
			r, err := client.Get(path, &res)
			if err != nil {
				output.Fail("request_failed", err.Error(), nil)
			}
			_ = output.PrintJSON(output.Envelope{Data: res, Meta: map[string]any{"status": r.StatusCode()}})
		},
	})

	return cmd
}
