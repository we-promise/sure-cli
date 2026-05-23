package root

import (
	"errors"

	"github.com/spf13/cobra"
	"github.com/we-promise/sure-cli/internal/api"
	"github.com/we-promise/sure-cli/internal/output"
)

type importPreflightOpts struct {
	File                      string
	RawFileContent            string
	Type                      string
	AccountID                 string
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
	RowsToSkip                string
}

func newImportsPreflightCmd() *cobra.Command {
	var o importPreflightOpts

	cmd := &cobra.Command{
		Use:   "preflight",
		Short: "Validate an import config + content before creating (read-only)",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			payload, err := buildImportPreflightPayload(o)
			if err != nil {
				output.Fail("validation_failed", err.Error(), nil)
				return
			}

			client := api.New()
			var res any
			var status int
			if payload.RawFileContent != "" {
				r, err := client.Post("/api/v1/imports/preflight", payload.Fields, &res)
				if err != nil {
					output.Fail("request_failed", err.Error(), nil)
				}
				status = r.StatusCode()
			} else {
				r, err := client.PostMultipart("/api/v1/imports/preflight", payload.Fields, payload.FileField, payload.FilePath, &res)
				if err != nil {
					output.Fail("request_failed", err.Error(), nil)
				}
				status = r.StatusCode()
			}
			if err := output.Print(format, output.Envelope{Data: res, Meta: &output.Meta{Status: status}}); err != nil {
				output.Fail("output_failed", err.Error(), nil)
			}
		},
	}

	cmd.Flags().StringVar(&o.File, "file", "", "path to import file")
	cmd.Flags().StringVar(&o.RawFileContent, "raw-file-content", "", "raw import file content")
	cmd.Flags().StringVar(&o.Type, "type", "", "import type (TransactionImport|SureImport); required")
	cmd.Flags().StringVar(&o.AccountID, "account-id", "", "account id (optional)")
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
	cmd.Flags().StringVar(&o.RowsToSkip, "rows-to-skip", "", "rows to skip at the top of the CSV")

	return cmd
}

func buildImportPreflightPayload(o importPreflightOpts) (importCreatePayload, error) {
	if o.Type == "" {
		return importCreatePayload{}, errors.New("type is required (e.g. TransactionImport, SureImport)")
	}
	if o.File == "" && o.RawFileContent == "" {
		return importCreatePayload{}, errors.New("file or raw-file-content is required")
	}
	if o.File != "" && o.RawFileContent != "" {
		return importCreatePayload{}, errors.New("provide only one of file or raw-file-content")
	}

	fields := map[string]string{"type": o.Type}
	if o.RawFileContent != "" {
		fields["raw_file_content"] = o.RawFileContent
	}
	addImportField(fields, "account_id", o.AccountID)
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
	addImportField(fields, "rows_to_skip", o.RowsToSkip)

	return importCreatePayload{
		FileField:      "file",
		FilePath:       o.File,
		RawFileContent: o.RawFileContent,
		Fields:         fields,
	}, nil
}
