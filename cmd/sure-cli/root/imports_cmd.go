package root

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-resty/resty/v2"
	"github.com/spf13/cobra"
	"github.com/we-promise/sure-cli/internal/api"
	"github.com/we-promise/sure-cli/internal/output"
)

type importCreateOpts struct {
	File                      string
	RawFileContent            string
	Type                      string
	Source                    string
	AccountID                 string
	FileFormat                string
	Publish                   bool
	DateColLabel              string
	AmountColLabel            string
	NameColLabel              string
	CategoryColLabel          string
	TagsColLabel              string
	NotesColLabel             string
	AccountColLabel           string
	QtyColLabel               string
	TickerColLabel            string
	PriceColLabel             string
	EntityTypeColLabel        string
	CurrencyColLabel          string
	ExchangeOperatingMICLabel string
	DateFormat                string
	NumberFormat              string
	SignageConvention         string
	ColSep                    string
	AmountTypeStrategy        string
	AmountTypeInflowValue     string
	Apply                     bool
}

func newImportsCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "imports", Short: "Imports"}

	var status, importType string
	var page, perPage int

	list := &cobra.Command{
		Use:   "list",
		Short: "List imports",
		Run: func(cmd *cobra.Command, args []string) {
			q := url.Values{}
			if status != "" {
				q.Set("status", status)
			}
			if importType != "" {
				q.Set("type", importType)
			}
			addPagingQuery(q, page, perPage)
			printGet(pathWithQuery("/api/v1/imports", q))
		},
	}

	list.Flags().StringVar(&status, "status", "", "filter by status")
	list.Flags().StringVar(&importType, "type", "", "filter by import type")
	addPagingFlags(list, &page, &perPage)
	cmd.AddCommand(list)

	cmd.AddCommand(&cobra.Command{
		Use:   "show <id>",
		Short: "Show import",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			printGet(fmt.Sprintf("/api/v1/imports/%s", url.PathEscape(args[0])))
		},
	})

	cmd.AddCommand(newImportsCreateCmd())
	cmd.AddCommand(newImportsRowsCmd())
	cmd.AddCommand(newImportsPreflightCmd())

	return cmd
}

func newImportsRowsCmd() *cobra.Command {
	var page, perPage int

	cmd := &cobra.Command{
		Use:   "rows <id>",
		Short: "List import row diagnostics",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			q := url.Values{}
			addPagingQuery(q, page, perPage)
			printGet(pathWithQuery(fmt.Sprintf("/api/v1/imports/%s/rows", url.PathEscape(args[0])), q))
		},
	}
	addPagingFlags(cmd, &page, &perPage)
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
			var r *resty.Response
			if payload.RawFileContent != "" {
				r, err = client.Post("/api/v1/imports", payload.Fields, &res)
			} else {
				r, err = client.PostMultipart("/api/v1/imports", payload.Fields, payload.FileField, payload.FilePath, &res)
			}
			respond(r, err, res)
		},
	}

	cmd.Flags().StringVar(&o.File, "file", "", "path to import file")
	cmd.Flags().StringVar(&o.RawFileContent, "raw-file-content", "", "raw import file content")
	cmd.Flags().StringVar(&o.Type, "type", "", "import type (TransactionImport|SureImport)")
	cmd.Flags().StringVar(&o.FileFormat, "file-format", "", "legacy import format hint")
	cmd.Flags().StringVar(&o.Source, "source", "", "legacy import source hint")
	cmd.Flags().StringVar(&o.AccountID, "account-id", "", "account id (optional)")
	cmd.Flags().BoolVar(&o.Publish, "publish", false, "queue processing when the import is configured")
	cmd.Flags().StringVar(&o.DateColLabel, "date-col-label", "", "CSV date column label")
	cmd.Flags().StringVar(&o.AmountColLabel, "amount-col-label", "", "CSV amount column label")
	cmd.Flags().StringVar(&o.NameColLabel, "name-col-label", "", "CSV name column label")
	cmd.Flags().StringVar(&o.CategoryColLabel, "category-col-label", "", "CSV category column label")
	cmd.Flags().StringVar(&o.TagsColLabel, "tags-col-label", "", "CSV tags column label")
	cmd.Flags().StringVar(&o.NotesColLabel, "notes-col-label", "", "CSV notes column label")
	cmd.Flags().StringVar(&o.AccountColLabel, "account-col-label", "", "CSV account column label")
	cmd.Flags().StringVar(&o.QtyColLabel, "qty-col-label", "", "CSV quantity column label")
	cmd.Flags().StringVar(&o.TickerColLabel, "ticker-col-label", "", "CSV ticker column label")
	cmd.Flags().StringVar(&o.PriceColLabel, "price-col-label", "", "CSV price column label")
	cmd.Flags().StringVar(&o.EntityTypeColLabel, "entity-type-col-label", "", "CSV entity type column label")
	cmd.Flags().StringVar(&o.CurrencyColLabel, "currency-col-label", "", "CSV currency column label")
	cmd.Flags().StringVar(&o.ExchangeOperatingMICLabel, "exchange-operating-mic-col-label", "", "CSV exchange operating MIC column label")
	cmd.Flags().StringVar(&o.DateFormat, "date-format", "", "CSV date format")
	cmd.Flags().StringVar(&o.NumberFormat, "number-format", "", "CSV number format")
	cmd.Flags().StringVar(&o.SignageConvention, "signage-convention", "", "CSV signage convention")
	cmd.Flags().StringVar(&o.ColSep, "col-sep", "", "CSV column separator")
	cmd.Flags().StringVar(&o.AmountTypeStrategy, "amount-type-strategy", "", "CSV amount type strategy")
	cmd.Flags().StringVar(&o.AmountTypeInflowValue, "amount-type-inflow-value", "", "CSV amount type inflow value")
	cmd.Flags().BoolVar(&o.Apply, "apply", false, "execute the create (otherwise dry-run)")

	return cmd
}

type importCreatePayload struct {
	FileField      string
	FilePath       string
	RawFileContent string
	Fields         map[string]string
}

func buildImportCreatePayload(o importCreateOpts) (importCreatePayload, error) {
	if o.File == "" && o.RawFileContent == "" {
		return importCreatePayload{}, errors.New("file or raw-file-content is required")
	}
	if o.File != "" && o.RawFileContent != "" {
		return importCreatePayload{}, errors.New("provide only one of file or raw-file-content")
	}

	if o.File != "" {
		info, err := os.Stat(o.File)
		if err != nil {
			return importCreatePayload{}, fmt.Errorf("file not accessible: %w", err)
		}
		if info.IsDir() {
			return importCreatePayload{}, errors.New("file must be a regular file")
		}
	}

	fileFormat := o.FileFormat
	if fileFormat == "" && o.File != "" {
		ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(o.File)), ".")
		if ext != "" {
			fileFormat = ext
		}
	}

	fields := map[string]string{}
	importType := o.Type
	if importType == "" && (fileFormat == "ndjson" || fileFormat == "json") {
		importType = "SureImport"
	}
	if importType != "" {
		fields["type"] = importType
	}
	if fileFormat != "" {
		fields["format"] = fileFormat
	}
	if o.RawFileContent != "" {
		fields["raw_file_content"] = o.RawFileContent
	}
	if o.Source != "" {
		fields["source"] = o.Source
	}
	if o.AccountID != "" {
		fields["account_id"] = o.AccountID
	}
	if o.Publish {
		fields["publish"] = "true"
	}
	addImportField(fields, "date_col_label", o.DateColLabel)
	addImportField(fields, "amount_col_label", o.AmountColLabel)
	addImportField(fields, "name_col_label", o.NameColLabel)
	addImportField(fields, "category_col_label", o.CategoryColLabel)
	addImportField(fields, "tags_col_label", o.TagsColLabel)
	addImportField(fields, "notes_col_label", o.NotesColLabel)
	addImportField(fields, "account_col_label", o.AccountColLabel)
	addImportField(fields, "qty_col_label", o.QtyColLabel)
	addImportField(fields, "ticker_col_label", o.TickerColLabel)
	addImportField(fields, "price_col_label", o.PriceColLabel)
	addImportField(fields, "entity_type_col_label", o.EntityTypeColLabel)
	addImportField(fields, "currency_col_label", o.CurrencyColLabel)
	addImportField(fields, "exchange_operating_mic_col_label", o.ExchangeOperatingMICLabel)
	addImportField(fields, "date_format", o.DateFormat)
	addImportField(fields, "number_format", o.NumberFormat)
	addImportField(fields, "signage_convention", o.SignageConvention)
	addImportField(fields, "col_sep", o.ColSep)
	addImportField(fields, "amount_type_strategy", o.AmountTypeStrategy)
	addImportField(fields, "amount_type_inflow_value", o.AmountTypeInflowValue)

	return importCreatePayload{
		FileField:      "file",
		FilePath:       o.File,
		RawFileContent: o.RawFileContent,
		Fields:         fields,
	}, nil
}

func addImportField(fields map[string]string, name, value string) {
	if value != "" {
		fields[name] = value
	}
}
