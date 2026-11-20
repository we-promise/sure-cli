package root

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/we-promise/sure-cli/internal/api"
	"github.com/we-promise/sure-cli/internal/models"
)

type txCreateOpts struct {
	AccountID    string
	Date         string
	Amount       string
	Nature       string
	Name         string
	Notes        string
	Currency     string
	CategoryID   string
	MerchantID   string
	TagIDs       []string
	ExternalID   string
	Source       string
	UserModified bool
	Apply        bool
}

func newTransactionsCreateCmd() *cobra.Command {
	var o txCreateOpts

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a transaction (default dry-run; use --apply to execute)",
		Run: func(cmd *cobra.Command, args []string) {
			if o.Date == "" {
				o.Date = familyToday(api.New())
			}
			payload, err := buildTxCreatePayload(o)
			failValidation(err)
			dispatchWrite(o.Apply, "POST", "/api/v1/transactions", payload)
		},
	}

	cmd.Flags().StringVar(&o.AccountID, "account-id", "", "account id (required)")
	cmd.Flags().StringVar(&o.Date, "date", "", familyDateUsage)
	addTxAmountFlags(cmd, &o.Amount, &o.Nature, "expense")
	cmd.Flags().StringVar(&o.Name, "name", "", "name/description (required)")
	cmd.Flags().StringVar(&o.Notes, "notes", "", "notes")
	cmd.Flags().StringVar(&o.Currency, "currency", "", "currency (default family currency)")
	addTxReferenceFlags(cmd, &o.CategoryID, &o.MerchantID, &o.TagIDs, &o.UserModified)
	cmd.Flags().StringVar(&o.ExternalID, "external-id", "", "idempotency key: a retry with the same external id (and source) returns the existing transaction")
	cmd.Flags().StringVar(&o.Source, "source", "", "namespace for --external-id (Sure defaults it to \"api\"; requires --external-id)")
	cmd.Flags().BoolVar(&o.Apply, "apply", false, "execute the create (otherwise dry-run)")

	_ = cmd.MarkFlagRequired("account-id")
	_ = cmd.MarkFlagRequired("name")

	return cmd
}

func buildTxCreatePayload(o txCreateOpts) (map[string]any, error) {
	if o.AccountID == "" {
		return nil, errors.New("account-id is required")
	}
	if o.Name == "" {
		return nil, errors.New("name is required")
	}
	if err := validateTxAmount(o.Amount); err != nil {
		return nil, err
	}
	if _, err := parseISODate("date", o.Date); err != nil {
		return nil, err
	}
	if err := validateTxNature(o.Nature); err != nil {
		return nil, err
	}
	if o.Source != "" && o.ExternalID == "" {
		return nil, errors.New("source requires external-id")
	}

	tx := map[string]any{
		"account_id": o.AccountID,
		"date":       o.Date,
		"amount":     o.Amount,
		"nature":     o.Nature,
		"name":       o.Name,
	}
	addAny(tx, "notes", o.Notes)
	addAny(tx, "currency", o.Currency)
	addAny(tx, "external_id", o.ExternalID)
	addAny(tx, "source", o.Source)
	addTxReferenceFields(tx, o.CategoryID, o.MerchantID, o.TagIDs, o.UserModified)

	// Rails expects { transaction: {...} }
	return map[string]any{"transaction": tx}, nil
}

// addTxAmountFlags registers --amount and --nature, shared by create and update.
func addTxAmountFlags(cmd *cobra.Command, amount, nature *string, defaultNature string) {
	cmd.Flags().StringVar(amount, "amount", "", "positive decimal amount, at most 4 decimals and 15 digits (e.g. 12.3456); the sign comes from --nature")
	cmd.Flags().StringVar(nature, "nature", defaultNature, "expense|income|outflow|inflow (maps to Sure transaction.nature)")
}

// addTxReferenceFlags registers the category/merchant/tag/user-modified
// flags shared by create and update.
func addTxReferenceFlags(cmd *cobra.Command, categoryID, merchantID *string, tagIDs *[]string, userModified *bool) {
	cmd.Flags().StringVar(categoryID, "category-id", "", "category id")
	cmd.Flags().StringVar(merchantID, "merchant-id", "", "merchant id")
	cmd.Flags().StringArrayVar(tagIDs, "tag-id", nil, "tag id (repeatable)")
	cmd.Flags().BoolVar(userModified, "user-modified", false, "protect the transaction from provider syncs, like a manual edit")
}

func addTxReferenceFields(tx map[string]any, categoryID, merchantID string, tagIDs []string, userModified bool) {
	addAny(tx, "category_id", categoryID)
	addAny(tx, "merchant_id", merchantID)
	if len(tagIDs) > 0 {
		tx["tag_ids"] = tagIDs
	}
	if userModified {
		tx["user_modified"] = true
	}
}

// maxTxAmountDigits bounds the digits of a transaction amount: Sure reads
// it with String#to_f, and a float keeps 15 significant digits exactly.
const maxTxAmountDigits = 15

// validateTxAmount accepts a positive decimal that Sure stores as typed. The
// text is sent verbatim, but Sure's TransactionsController reads it with
// to_f into decimal(19,4): more than 4 decimals would be rounded away and
// more than 15 digits would not survive the float.
func validateTxAmount(amount string) error {
	if !models.IsPositiveDecimal(amount) {
		return fmt.Errorf("amount must be a positive decimal like 12.34 (got %q); the sign comes from --nature", amount)
	}
	if err := moneyColumn.check("amount", amount); err != nil {
		return err
	}
	if models.DecimalIntegerDigits(amount)+models.DecimalScale(amount) > maxTxAmountDigits {
		return fmt.Errorf("amount has more than %d digits, which Sure cannot store exactly, got %q", maxTxAmountDigits, amount)
	}
	return nil
}

func validateTxNature(nature string) error {
	switch nature {
	case "income", "expense", "inflow", "outflow":
		return nil
	default:
		return errors.New("nature must be one of: income|expense|inflow|outflow")
	}
}
