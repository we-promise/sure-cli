package root

import (
	"fmt"
	"net/url"

	"github.com/spf13/cobra"
)

type txDeleteOpts struct {
	Apply bool
}

func newTransactionsDeleteCmd() *cobra.Command {
	var o txDeleteOpts

	cmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a transaction (default dry-run; use --apply to execute)",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			dispatchWrite(o.Apply, "DELETE", fmt.Sprintf("/api/v1/transactions/%s", url.PathEscape(args[0])), nil)
		},
	}

	cmd.Flags().BoolVar(&o.Apply, "apply", false, "execute the delete (otherwise dry-run)")
	return cmd
}
