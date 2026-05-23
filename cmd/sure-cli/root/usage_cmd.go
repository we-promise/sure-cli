package root

import (
	"github.com/spf13/cobra"
)

func newUsageCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "usage", Short: "Show API usage and rate-limit info for the current credential"}

	cmd.AddCommand(&cobra.Command{
		Use:   "show",
		Short: "Show usage info (singleton)",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			printGet("/api/v1/usage")
		},
	})

	return cmd
}
