package root

import (
	"github.com/spf13/cobra"
)

func newUsersCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "users", Short: "User account operations"}

	var applyReset bool
	reset := &cobra.Command{
		Use:   "reset",
		Short: "Queue account reset (default dry-run; use --apply to execute)",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			dispatchWrite(applyReset, "DELETE", "/api/v1/users/reset", nil)
		},
	}
	reset.Flags().BoolVar(&applyReset, "apply", false, "execute the reset (otherwise dry-run)")
	reset.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Show reset status",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			printGet("/api/v1/users/reset/status")
		},
	})
	cmd.AddCommand(reset)

	var applyDelete bool
	deleteMe := &cobra.Command{
		Use:   "delete-me",
		Short: "Delete current user account (default dry-run; use --apply to execute)",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			dispatchWrite(applyDelete, "DELETE", "/api/v1/users/me", nil)
		},
	}
	deleteMe.Flags().BoolVar(&applyDelete, "apply", false, "execute the account deletion (otherwise dry-run)")
	cmd.AddCommand(deleteMe)

	return cmd
}
