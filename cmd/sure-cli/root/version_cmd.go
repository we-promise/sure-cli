package root

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/we-promise/sure-cli/internal/api"
	"github.com/we-promise/sure-cli/internal/config"
	errs "github.com/we-promise/sure-cli/internal/errors"
)

// newVersionCmd prints the local build info. --server adds the running Sure
// server's version (GET /api/v1/version) as an envelope; a server that cannot
// be reached is reported under data.server_error instead of failing, because
// the local version is still valid output.
func newVersionCmd() *cobra.Command {
	var server bool
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print version information (--server adds the Sure server version)",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			if !server {
				fmt.Printf("sure-cli %s\n", version)
				fmt.Printf("  commit: %s\n", commit)
				fmt.Printf("  built:  %s\n", date)
				return
			}
			printEnvelope(buildVersionReport(fetchServerVersion()), nil)
		},
	}
	cmd.Flags().BoolVar(&server, "server", false, "also query the configured Sure server's version (JSON envelope)")
	return cmd
}

// fetchServerVersion returns the server's {version, commit_sha} payload, or a
// classified error carrying the same details respond() would attach.
func fetchServerVersion() (any, *errs.CLIError) {
	if config.APIURL() == "" {
		return nil, errs.New(errs.CodeConfigMissing, "api_url is not configured")
	}
	var res any
	if ce := api.CheckResponse(api.New().Get("/api/v1/version", &res)); ce != nil {
		return nil, ce
	}
	return res, nil
}

func buildVersionReport(server any, serverErr *errs.CLIError) map[string]any {
	report := map[string]any{
		"cli": map[string]any{"version": version, "commit": commit, "built": date},
	}
	if serverErr != nil {
		report["server_error"] = serverErr
	} else {
		report["server"] = server
	}
	return report
}
