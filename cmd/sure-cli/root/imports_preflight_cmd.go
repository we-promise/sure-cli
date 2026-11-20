package root

import (
	"errors"

	"github.com/spf13/cobra"
	"github.com/we-promise/sure-cli/internal/api"
)

type importPreflightOpts struct {
	File           string
	RawFileContent string
	Type           string
	importConfig
}

func newImportsPreflightCmd() *cobra.Command {
	var o importPreflightOpts

	cmd := &cobra.Command{
		Use:   "preflight",
		Short: "Validate an import config + content before creating (read-only)",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			payload, err := buildImportPreflightPayload(o)
			failValidation(err)
			var res any
			r, err := payload.send(api.New(), "/api/v1/imports/preflight", &res)
			respond(r, err, res)
		},
	}

	cmd.Flags().StringVar(&o.File, "file", "", "path to import file")
	cmd.Flags().StringVar(&o.RawFileContent, "raw-file-content", "", "raw import file content")
	cmd.Flags().StringVar(&o.Type, "type", "", "import type (TransactionImport|SureImport); required")
	addImportConfigFlags(cmd, &o.importConfig)

	return cmd
}

func buildImportPreflightPayload(o importPreflightOpts) (upload, error) {
	if o.Type == "" {
		return upload{}, errors.New("type is required (e.g. TransactionImport, SureImport)")
	}
	return o.importConfig.upload(o.File, o.RawFileContent, o.Type)
}
