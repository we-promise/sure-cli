package root

import (
	"fmt"
	"net/url"

	"github.com/spf13/cobra"
)

func newSyncsCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "syncs", Short: "List and inspect sync history"}

	var page, perPage int
	list := &cobra.Command{
		Use:   "list",
		Short: "List syncs (ordered most-recent first)",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			q := url.Values{}
			addPagingQuery(q, page, perPage)
			printGet(pathWithQuery("/api/v1/syncs", q))
		},
	}
	addPagingFlags(list, &page, &perPage)
	cmd.AddCommand(list)

	cmd.AddCommand(&cobra.Command{
		Use:   "latest",
		Short: "Show the most recent sync (data:null if none)",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			printGet("/api/v1/syncs/latest")
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "show <id>",
		Short: "Show a sync by id",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			printGet(fmt.Sprintf("/api/v1/syncs/%s", url.PathEscape(args[0])))
		},
	})

	return cmd
}
