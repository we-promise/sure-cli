package root

import (
	"github.com/dgilperez/sure-cli/internal/api"
	"github.com/dgilperez/sure-cli/internal/output"
	"github.com/spf13/cobra"
)

// NOTE: Sure doesn't expose /whoami yet. We use /api/v1/usage as a proxy.
// If authenticated via API key, it returns api_key info. If OAuth, it returns method.
func newWhoamiCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "whoami",
		Short: "Show current auth context",
		Run: func(cmd *cobra.Command, args []string) {
			client := api.New()
			var res any
			r, err := client.Get("/api/v1/usage", &res)
			if err != nil {
				output.Fail("request_failed", err.Error(), nil)
			}
			_ = output.PrintJSON(output.Envelope{Data: res, Meta: map[string]any{"status": r.StatusCode()}})
		},
	}
}
