package root

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/we-promise/sure-cli/internal/api"
	"github.com/we-promise/sure-cli/internal/output"
)

type importCreateOpts struct {
	File       string
	Source     string
	AccountID  string
	FileFormat string
	Apply      bool
}

func newImportsCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "imports", Short: "Imports"}

	var status, accountID string
	var page, perPage int
	var limit int

	list := &cobra.Command{
		Use:   "list",
		Short: "List imports",
		Run: func(cmd *cobra.Command, args []string) {
			client := api.New()

			q := url.Values{}
			if status != "" {
				q.Set("status", status)
			}
			if accountID != "" {
				q.Set("account_id", accountID)
			}
			if page > 0 {
				q.Set("page", fmt.Sprintf("%d", page))
			}
			if perPage > 0 {
				q.Set("per_page", fmt.Sprintf("%d", perPage))
			}
			if limit > 0 {
				q.Set("limit", fmt.Sprintf("%d", limit))
			}

			u := url.URL{Path: "/api/v1/imports", RawQuery: q.Encode()}
			path := u.String()

			var res any
			r, err := client.Get(path, &res)
			if err != nil {
				output.Fail("request_failed", err.Error(), nil)
			}
			if err := output.Print(format, output.Envelope{Data: res, Meta: &output.Meta{Status: r.StatusCode()}}); err != nil {
				output.Fail("output_failed", err.Error(), nil)
			}
		},
	}

	list.Flags().StringVar(&status, "status", "", "filter by status")
	list.Flags().StringVar(&accountID, "account-id", "", "filter by account id")
	list.Flags().IntVar(&page, "page", 1, "page number")
	list.Flags().IntVar(&perPage, "per-page", 25, "items per page (maps to per_page)")
	list.Flags().IntVar(&limit, "limit", 50, "max results")
	cmd.AddCommand(list)

	cmd.AddCommand(&cobra.Command{
		Use:   "show <id>",
		Short: "Show import",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			client := api.New()
			var res any
			path := fmt.Sprintf("/api/v1/imports/%s", args[0])
			r, err := client.Get(path, &res)
			if err != nil {
				output.Fail("request_failed", err.Error(), nil)
			}
			if err := output.Print(format, output.Envelope{Data: res, Meta: &output.Meta{Status: r.StatusCode()}}); err != nil {
				output.Fail("output_failed", err.Error(), nil)
			}
		},
	})

	cmd.AddCommand(newImportsCreateCmd())
	cmd.AddCommand(newImportsDeleteCmd())

	return cmd
}

func newImportsCreateCmd() *cobra.Command {
	var o importCreateOpts

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create an import (default dry-run; use --apply to execute)",
		Run: func(cmd *cobra.Command, args []string) {
			payload, err := buildImportCreatePayload(o)
			if err != nil {
				output.Fail("validation_failed", err.Error(), nil)
			}

			if !o.Apply {
				if err := output.Print(format, output.Envelope{Data: map[string]any{
					"dry_run": true,
					"request": map[string]any{
						"method": "POST",
						"path":   "/api/v1/imports",
						"body":   payload,
					},
				}}); err != nil {
					output.Fail("output_failed", err.Error(), nil)
				}
				return
			}

			client := api.New()
			var res any
			r, err := client.PostMultipart("/api/v1/imports", payload.Fields, payload.FileField, payload.FilePath, &res)
			if err != nil {
				output.Fail("request_failed", err.Error(), nil)
			}
			if err := output.Print(format, output.Envelope{Data: res, Meta: &output.Meta{Status: r.StatusCode()}}); err != nil {
				output.Fail("output_failed", err.Error(), nil)
			}
		},
	}

	cmd.Flags().StringVar(&o.File, "file", "", "path to import file (required)")
	cmd.Flags().StringVar(&o.FileFormat, "file-format", "", "import format (e.g. csv|ofx)")
	cmd.Flags().StringVar(&o.Source, "source", "", "import source (optional)")
	cmd.Flags().StringVar(&o.AccountID, "account-id", "", "account id (optional)")
	cmd.Flags().BoolVar(&o.Apply, "apply", false, "execute the create (otherwise dry-run)")

	if err := cmd.MarkFlagRequired("file"); err != nil {
		panic(err)
	}

	return cmd
}

func newImportsDeleteCmd() *cobra.Command {
	var apply bool

	cmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete an import (default dry-run; use --apply to execute)",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			id := args[0]
			path := fmt.Sprintf("/api/v1/imports/%s", id)

			if !apply {
				if err := output.Print(format, output.Envelope{Data: map[string]any{
					"dry_run": true,
					"request": map[string]any{
						"method": "DELETE",
						"path":   path,
					},
				}}); err != nil {
					output.Fail("output_failed", err.Error(), nil)
				}
				return
			}

			client := api.New()
			var res any
			r, err := client.Delete(path, &res)
			if err != nil {
				output.Fail("request_failed", err.Error(), nil)
			}
			if err := output.Print(format, output.Envelope{Data: res, Meta: &output.Meta{Status: r.StatusCode()}}); err != nil {
				output.Fail("output_failed", err.Error(), nil)
			}
		},
	}

	cmd.Flags().BoolVar(&apply, "apply", false, "execute the delete (otherwise dry-run)")
	return cmd
}

type importCreatePayload struct {
	FileField string
	FilePath  string
	Fields    map[string]string
}

func buildImportCreatePayload(o importCreateOpts) (importCreatePayload, error) {
	if o.File == "" {
		return importCreatePayload{}, errors.New("file is required")
	}
	info, err := os.Stat(o.File)
	if err != nil {
		return importCreatePayload{}, fmt.Errorf("file not accessible: %w", err)
	}
	if info.IsDir() {
		return importCreatePayload{}, errors.New("file must be a regular file")
	}

	fileFormat := o.FileFormat
	if fileFormat == "" {
		ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(o.File)), ".")
		if ext != "" {
			fileFormat = ext
		}
	}

	fields := map[string]string{}
	if fileFormat != "" {
		fields["format"] = fileFormat
	}
	if o.Source != "" {
		fields["source"] = o.Source
	}
	if o.AccountID != "" {
		fields["account_id"] = o.AccountID
	}

	return importCreatePayload{
		FileField: "file",
		FilePath:  o.File,
		Fields:    fields,
	}, nil
}
