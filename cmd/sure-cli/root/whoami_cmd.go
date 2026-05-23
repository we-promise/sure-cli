package root

import (
	"github.com/spf13/cobra"

	"github.com/we-promise/sure-cli/internal/api"
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
			respond(r, err, res)
		},
	}
}
