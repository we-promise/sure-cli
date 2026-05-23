package root

import (
	"github.com/spf13/cobra"
	"github.com/we-promise/sure-cli/internal/api"
	"github.com/we-promise/sure-cli/internal/output"
)

func newUsersCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "users", Short: "User account operations"}

	var applyReset bool
	reset := &cobra.Command{
		Use:   "reset",
		Short: "Queue account reset (default dry-run; use --apply to execute)",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			path := "/api/v1/users/reset"
			if !applyReset {
				printUsersDryRun("DELETE", path, nil)
				return
			}
			printUsersDelete(path)
		},
	}
	reset.Flags().BoolVar(&applyReset, "apply", false, "execute the reset (otherwise dry-run)")
	reset.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Show reset status",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			printUsersGet("/api/v1/users/reset/status")
		},
	})
	cmd.AddCommand(reset)

	var applyDelete bool
	deleteMe := &cobra.Command{
		Use:   "delete-me",
		Short: "Delete current user account (default dry-run; use --apply to execute)",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			path := "/api/v1/users/me"
			if !applyDelete {
				printUsersDryRun("DELETE", path, nil)
				return
			}
			printUsersDelete(path)
		},
	}
	deleteMe.Flags().BoolVar(&applyDelete, "apply", false, "execute the account deletion (otherwise dry-run)")
	cmd.AddCommand(deleteMe)

	return cmd
}

func printUsersGet(path string) {
	client := api.New()
	var res any
	r, err := client.Get(path, &res)
	if err != nil {
		output.Fail("request_failed", err.Error(), nil)
		return
	}
	if err := output.Print(format, output.Envelope{Data: res, Meta: &output.Meta{Status: r.StatusCode()}}); err != nil {
		output.Fail("output_failed", err.Error(), nil)
		return
	}
}

func printUsersDelete(path string) {
	client := api.New()
	var res any
	r, err := client.Delete(path, &res)
	if err != nil {
		output.Fail("request_failed", err.Error(), nil)
		return
	}
	if err := output.Print(format, output.Envelope{Data: res, Meta: &output.Meta{Status: r.StatusCode()}}); err != nil {
		output.Fail("output_failed", err.Error(), nil)
		return
	}
}

func printUsersDryRun(method, path string, body any) {
	request := map[string]any{
		"method": method,
		"path":   path,
	}
	if body != nil {
		request["body"] = body
	}
	if err := output.Print(format, output.Envelope{Data: map[string]any{
		"dry_run": true,
		"request": request,
	}}); err != nil {
		output.Fail("output_failed", err.Error(), nil)
		return
	}
}
