package root

import (
	"github.com/spf13/cobra"
)

func newAuthCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "auth", Short: "Auth-related write operations on the current account"}
	cmd.AddCommand(newAuthEnableAICmd())
	return cmd
}

func newAuthEnableAICmd() *cobra.Command {
	var apply bool
	cmd := &cobra.Command{
		Use:   "enable-ai",
		Short: "Enable AI on the current account (PATCH /api/v1/auth/enable_ai; default dry-run; use --apply to execute)",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			// upstream auth#enable_ai ignores the request body; send {} on apply.
			dispatchWrite(apply, "PATCH", "/api/v1/auth/enable_ai", map[string]any{})
		},
	}
	cmd.Flags().BoolVar(&apply, "apply", false, "execute the enable (otherwise dry-run)")
	return cmd
}
