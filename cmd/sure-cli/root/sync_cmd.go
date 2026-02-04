package root

import (
	"github.com/dgilperez/sure-cli/internal/api"
	"github.com/dgilperez/sure-cli/internal/output"
	"github.com/spf13/cobra"
)

func newSyncCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "sync",
		Short: "Trigger sync",
		Run: func(cmd *cobra.Command, args []string) {
			client := api.New()
			var res any
			r, err := client.Post("/api/v1/sync", map[string]any{}, &res)
			if err != nil {
				output.Fail("request_failed", err.Error(), nil)
			}
			_ = output.Print(format, output.Envelope{Data: res, Meta: map[string]any{"status": r.StatusCode()}})
		},
	}
}
