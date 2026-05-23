package root

import (
	"github.com/spf13/cobra"
)

func newProviderConnectionsCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "provider-connections", Short: "Aggregator / data-provider connection status"}

	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List provider connections for the current family",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			printGet("/api/v1/provider_connections")
		},
	})

	return cmd
}
