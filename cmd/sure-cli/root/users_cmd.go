package root

import "github.com/spf13/cobra"

func newUsersCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "users", Short: "User account operations"}

	var applyReset bool
	reset := &cobra.Command{
		Use:   "reset",
		Short: "Queue account reset (default dry-run; use --apply to execute)",
		Run: func(cmd *cobra.Command, args []string) {
			path := "/api/v1/users/reset"
			if !applyReset {
				printDryRun("DELETE", path, nil)
				return
			}
			printDelete(path)
		},
	}
	reset.Flags().BoolVar(&applyReset, "apply", false, "execute the reset (otherwise dry-run)")
	reset.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Show reset status",
		Run: func(cmd *cobra.Command, args []string) {
			printGet("/api/v1/users/reset/status")
		},
	})
	cmd.AddCommand(reset)

	var applyDelete bool
	deleteMe := &cobra.Command{
		Use:   "delete-me",
		Short: "Delete current user account (default dry-run; use --apply to execute)",
		Run: func(cmd *cobra.Command, args []string) {
			path := "/api/v1/users/me"
			if !applyDelete {
				printDryRun("DELETE", path, nil)
				return
			}
			printDelete(path)
		},
	}
	deleteMe.Flags().BoolVar(&applyDelete, "apply", false, "execute the account deletion (otherwise dry-run)")
	cmd.AddCommand(deleteMe)

	return cmd
}
