package root

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/we-promise/sure-cli/internal/api"
)

type importCreateOpts struct {
	File           string
	RawFileContent string
	Type           string
	Publish        bool
	Apply          bool
	importConfig
}

// importConfig is the account, column mapping and parsing options that
// imports create and imports preflight both send.
type importConfig struct {
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
			failValidation(err)
			if !o.Apply {
				printDryRun("POST", "/api/v1/imports", payload.dryRunBody())
				return
			}

			var res any
			r, err := payload.send(api.New(), "/api/v1/imports", &res)
			respond(r, err, res)
		},
	}

	cmd.Flags().StringVar(&o.File, "file", "", "path to import file")
	cmd.Flags().StringVar(&o.RawFileContent, "raw-file-content", "", "raw import file content")
	cmd.Flags().StringVar(&o.Type, "type", "", "import type (TransactionImport|SureImport; a .ndjson/.json --file defaults to SureImport)")
	cmd.Flags().BoolVar(&o.Publish, "publish", false, "queue processing when the import is configured")
	addImportConfigFlags(cmd, &o.importConfig)
	cmd.Flags().BoolVar(&o.Apply, "apply", false, "execute the create (otherwise dry-run)")

	return cmd
}

func buildImportCreatePayload(o importCreateOpts) (upload, error) {
	importType := o.Type
	if importType == "" && o.File != "" {
		switch strings.ToLower(filepath.Ext(o.File)) {
		case ".ndjson", ".json":
			importType = "SureImport"
		}
	}
	u, err := o.importConfig.upload(o.File, o.RawFileContent, importType)
	if err != nil {
		return upload{}, err
	}
	if o.Publish {
		u.Fields["publish"] = "true"
	}
	return u, nil
}

// upload validates the content source and rows-to-skip and builds the
// request fields.
func (c importConfig) upload(file, raw, importType string) (upload, error) {
	if err := checkUploadContent(file, raw, "raw-file-content"); err != nil {
		return upload{}, err
	}
	if c.RowsToSkip != "" {
		if n, err := strconv.Atoi(c.RowsToSkip); err != nil || n < 0 {
			return upload{}, fmt.Errorf("rows-to-skip must be a non-negative integer (got %q)", c.RowsToSkip)
		}
	}
	fields := map[string]string{}
	for name, value := range map[string]string{
		"type":                             importType,
		"account_id":                       c.AccountID,
		"date_col_label":                   c.DateColLabel,
		"amount_col_label":                 c.AmountColLabel,
		"name_col_label":                   c.NameColLabel,
		"category_col_label":               c.CategoryColLabel,
		"tags_col_label":                   c.TagsColLabel,
		"notes_col_label":                  c.NotesColLabel,
		"account_col_label":                c.AccountColLabel,
		"qty_col_label":                    c.QtyColLabel,
		"ticker_col_label":                 c.TickerColLabel,
		"price_col_label":                  c.PriceColLabel,
		"entity_type_col_label":            c.EntityTypeColLabel,
		"currency_col_label":               c.CurrencyColLabel,
		"exchange_operating_mic_col_label": c.ExchangeOperatingMICLabel,
		"date_format":                      c.DateFormat,
		"number_format":                    c.NumberFormat,
		"signage_convention":               c.SignageConvention,
		"col_sep":                          c.ColSep,
		"amount_type_strategy":             c.AmountTypeStrategy,
		"amount_type_inflow_value":         c.AmountTypeInflowValue,
		"rows_to_skip":                     c.RowsToSkip,
	} {
		addImportField(fields, name, value)
	}
	return upload{Fields: fields, FilePath: file, Raw: raw}, nil
}

func addImportConfigFlags(cmd *cobra.Command, c *importConfig) {
	cmd.Flags().StringVar(&c.AccountID, "account-id", "", "account id (optional)")
	cmd.Flags().StringVar(&c.DateColLabel, "date-col-label", "", "CSV date column label")
	cmd.Flags().StringVar(&c.AmountColLabel, "amount-col-label", "", "CSV amount column label")
	cmd.Flags().StringVar(&c.NameColLabel, "name-col-label", "", "CSV name column label")
	cmd.Flags().StringVar(&c.CategoryColLabel, "category-col-label", "", "CSV category column label")
	cmd.Flags().StringVar(&c.TagsColLabel, "tags-col-label", "", "CSV tags column label")
	cmd.Flags().StringVar(&c.NotesColLabel, "notes-col-label", "", "CSV notes column label")
	cmd.Flags().StringVar(&c.AccountColLabel, "account-col-label", "", "CSV account column label")
	cmd.Flags().StringVar(&c.QtyColLabel, "qty-col-label", "", "CSV quantity column label")
	cmd.Flags().StringVar(&c.TickerColLabel, "ticker-col-label", "", "CSV ticker column label")
	cmd.Flags().StringVar(&c.PriceColLabel, "price-col-label", "", "CSV price column label")
	cmd.Flags().StringVar(&c.EntityTypeColLabel, "entity-type-col-label", "", "CSV entity type column label")
	cmd.Flags().StringVar(&c.CurrencyColLabel, "currency-col-label", "", "CSV currency column label")
	cmd.Flags().StringVar(&c.ExchangeOperatingMICLabel, "exchange-operating-mic-col-label", "", "CSV exchange operating MIC column label")
	cmd.Flags().StringVar(&c.DateFormat, "date-format", "", "CSV date format")
	cmd.Flags().StringVar(&c.NumberFormat, "number-format", "", "CSV number format")
	cmd.Flags().StringVar(&c.SignageConvention, "signage-convention", "", "CSV signage convention")
	cmd.Flags().StringVar(&c.ColSep, "col-sep", "", "CSV column separator")
	cmd.Flags().StringVar(&c.AmountTypeStrategy, "amount-type-strategy", "", "CSV amount type strategy")
	cmd.Flags().StringVar(&c.AmountTypeInflowValue, "amount-type-inflow-value", "", "CSV amount type inflow value")
	cmd.Flags().StringVar(&c.RowsToSkip, "rows-to-skip", "", "number of leading CSV lines to drop before the header")
}

func addImportField(fields map[string]string, name, value string) {
	if value != "" {
		fields[name] = value
	}
}
