package root

import (
	"errors"
	"fmt"
	"net/url"

	"github.com/spf13/cobra"
)

// txUpdateOpts has no currency: Sure's entry_params_for_update ignores it.
type txUpdateOpts struct {
	ID           string
	Date         string
	Amount       string
	Nature       string
	Name         string
	Notes        string
	CategoryID   string
	MerchantID   string
	TagIDs       []string
	ClearTags    bool
	UserModified bool
	Apply        bool
}

func newTransactionsUpdateCmd() *cobra.Command {
	var o txUpdateOpts

	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update a transaction (default dry-run; use --apply to execute)",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			o.ID = args[0]
			payload, err := buildTxUpdatePayload(o)
			failValidation(err)
			dispatchWrite(o.Apply, "PATCH", fmt.Sprintf("/api/v1/transactions/%s", url.PathEscape(o.ID)), payload)
		},
	}

	cmd.Flags().StringVar(&o.Date, "date", "", "date YYYY-MM-DD")
	addTxAmountFlags(cmd, &o.Amount, &o.Nature, "")
	cmd.Flags().StringVar(&o.Name, "name", "", "name/description")
	cmd.Flags().StringVar(&o.Notes, "notes", "", "notes")
	addTxReferenceFlags(cmd, &o.CategoryID, &o.MerchantID, &o.TagIDs, &o.UserModified)
	cmd.Flags().BoolVar(&o.ClearTags, "clear-tags", false, "remove every tag (without --tag-id or --clear-tags the tags are kept)")
	cmd.Flags().BoolVar(&o.Apply, "apply", false, "execute the update (otherwise dry-run)")

	return cmd
}

func buildTxUpdatePayload(o txUpdateOpts) (map[string]any, error) {
	if o.ClearTags && len(o.TagIDs) > 0 {
		return nil, errors.New("use either --tag-id or --clear-tags, not both")
	}
	if o.Date != "" {
		if err := validateTxDate(o.Date); err != nil {
			return nil, err
		}
	}
	if o.Nature != "" && o.Amount == "" {
		return nil, errors.New("nature only applies with amount; Sure ignores it otherwise")
	}
	if o.Amount != "" {
		if err := validateTxAmount(o.Amount); err != nil {
			return nil, err
		}
		if o.Nature == "" {
			return nil, errors.New("nature is required when amount is provided")
		}
		if err := validateTxNature(o.Nature); err != nil {
			return nil, err
		}
	}

	tx := map[string]any{}
	addAny(tx, "date", o.Date)
	if o.Amount != "" {
		tx["amount"] = o.Amount
		tx["nature"] = o.Nature
	}
	addAny(tx, "name", o.Name)
	addAny(tx, "notes", o.Notes)
	addTxReferenceFields(tx, o.CategoryID, o.MerchantID, o.TagIDs, o.UserModified)
	if o.ClearTags {
		// An explicit empty array clears tags; an absent key keeps them.
		tx["tag_ids"] = []string{}
	}
	if len(tx) == 0 {
		return nil, errors.New("no fields provided to update")
	}

	return map[string]any{"transaction": tx}, nil
}
