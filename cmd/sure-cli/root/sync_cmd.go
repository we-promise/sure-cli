package root

import (
	"github.com/spf13/cobra"

	"github.com/we-promise/sure-cli/internal/api"
)

func newSyncCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "sync",
		Short: "Trigger sync",
		Run: func(cmd *cobra.Command, args []string) {
			client := api.New()
			var res any
			r, err := client.Post("/api/v1/sync", map[string]any{}, &res)
			respond(r, err, res)
		},
	}
}
