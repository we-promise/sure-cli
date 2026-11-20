package root

import (
	"github.com/spf13/cobra"
)

func newSyncCmd() *cobra.Command {
	var apply bool
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Trigger a family sync (default dry-run; use --apply to execute)",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			dispatchWrite(apply, "POST", "/api/v1/sync", map[string]any{})
		},
	}
	cmd.Flags().BoolVar(&apply, "apply", false, "trigger the sync (otherwise dry-run)")
	return cmd
}
