package root

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/url"
	"os"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

type txSplitOpts struct {
	Splits     string
	SplitsFile string
	Apply      bool
}

func newTransactionsSplitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "split",
		Short: "Split a transaction into child transactions that sum to it",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "show <transaction_id>",
		Short: "Show a transaction's split (a child id resolves to its parent)",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			printGet(txSplitPath(args[0]))
		},
	})

	cmd.AddCommand(newTxSplitWriteCmd(
		"create <transaction_id>",
		"Split a transaction (default dry-run; use --apply to execute)",
		"POST",
	))
	cmd.AddCommand(newTxSplitWriteCmd(
		"update <transaction_id>",
		"Replace every child of a split (default dry-run; use --apply to execute)",
		"PATCH",
	))

	var apply bool
	del := &cobra.Command{
		Use:   "delete <transaction_id>",
		Short: "Remove a split and restore the parent (default dry-run; use --apply to execute)",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			dispatchWrite(apply, "DELETE", txSplitPath(args[0]), nil)
		},
	}
	del.Flags().BoolVar(&apply, "apply", false, "execute the delete (otherwise dry-run)")
	cmd.AddCommand(del)

	return cmd
}

func newTxSplitWriteCmd(use, short, method string) *cobra.Command {
	var o txSplitOpts

	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Long: short + `

Children are a JSON array and must sum exactly to the transaction:

  [{"amount": "60.00", "name": "Groceries", "category_id": "<uuid>"},
   {"amount": "40.00", "excluded": true}]

Amounts take the sign of amount_decimal in "split show", or of amount in
"transactions show": expenses are positive. Not signed_amount_cents, which is
negative for expenses.

Every child is recreated, keeping only amount, name, category and excluded.
The output of "split show" can be saved, edited and passed back with
--splits-file. Edit amount_decimal there: amount is display-formatted, and a
plain amount that disagrees with amount_decimal is rejected. A child carrying
tags or notes is rejected too, since they would be lost; remove them first.`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			raw, err := readTxSplitInput(o, os.Stdin)
			if err != nil {
				failValidation(err)
				return
			}
			body, err := buildTxSplitPayload(raw)
			if err != nil {
				failValidation(err)
				return
			}
			dispatchWrite(o.Apply, method, txSplitPath(args[0]), body)
		},
	}

	cmd.Flags().StringVar(&o.Splits, "splits", "", "children as a JSON array")
	cmd.Flags().StringVar(&o.SplitsFile, "splits-file", "", "path to a JSON file of children (- for stdin)")
	cmd.Flags().BoolVar(&o.Apply, "apply", false, "execute the write (otherwise dry-run)")
	return cmd
}

func txSplitPath(id string) string {
	return fmt.Sprintf("/api/v1/transactions/%s/split", url.PathEscape(id))
}

func readTxSplitInput(o txSplitOpts, stdin io.Reader) ([]byte, error) {
	switch {
	case o.Splits != "" && o.SplitsFile != "":
		return nil, errors.New("provide only one of splits or splits-file")
	case o.Splits != "":
		return []byte(o.Splits), nil
	case o.SplitsFile == "-":
		return io.ReadAll(stdin)
	case o.SplitsFile != "":
		return os.ReadFile(o.SplitsFile)
	default:
		return nil, errors.New("splits or splits-file is required")
	}
}

// Amounts stay as the text the caller wrote. The server stores four decimal
// places and requires an exact sum, so a float round trip would break it.
var txSplitAmountRe = regexp.MustCompile(`^-?\d+(\.\d+)?$`)

var txSplitInputKeys = map[string]bool{
	"amount": true, "amount_decimal": true, "name": true,
	"category_id": true, "category": true, "excluded": true,
}

// buildTxSplitPayload accepts a bare array of children, or the object that
// "split show" prints (with or without its envelope).
func buildTxSplitPayload(raw []byte) (map[string]any, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var parsed any
	if err := dec.Decode(&parsed); err != nil {
		return nil, fmt.Errorf("splits is not valid JSON: %w", err)
	}

	if obj, ok := parsed.(map[string]any); ok {
		if data, ok := obj["data"].(map[string]any); ok {
			obj = data
		}
		parsed = obj["children"]
	}
	items, ok := parsed.([]any)
	if !ok || len(items) == 0 {
		return nil, errors.New("splits must be a non-empty JSON array")
	}

	splits := make([]map[string]any, 0, len(items))
	for i, item := range items {
		child, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("split %d must be an object", i+1)
		}
		split, err := buildTxSplitChild(child)
		if err != nil {
			return nil, fmt.Errorf("split %d: %w", i+1, err)
		}
		splits = append(splits, split)
	}

	return map[string]any{"split": map[string]any{"splits": splits}}, nil
}

func buildTxSplitChild(child map[string]any) (map[string]any, error) {
	// A child copied from "split show" is a whole transaction; its extra
	// fields are ignored. Anything else is checked so a typo is not dropped.
	if _, fromShow := child["id"]; !fromShow {
		for key := range child {
			if !txSplitInputKeys[key] {
				return nil, fmt.Errorf("unknown field %q", key)
			}
		}
	}

	if lost := txSplitLostFields(child); len(lost) > 0 {
		return nil, fmt.Errorf("has %s, which the split API does not keep on a child and would be lost; remove them from the input to proceed", strings.Join(lost, " and "))
	}

	// show's "amount" is display-formatted ("$60.00"); amount_decimal is exact.
	// When both are plain numbers and differ, either could be the edit, so
	// refuse rather than guess.
	amountValue, hasDecimal := child["amount_decimal"]
	display, hasAmount := child["amount"]
	if hasDecimal && hasAmount {
		shown, exact := txSplitAmountText(display), txSplitAmountText(amountValue)
		if txSplitAmountRe.MatchString(shown) && txSplitAmountRe.MatchString(exact) && !txSplitSameDecimal(shown, exact) {
			return nil, fmt.Errorf("amount (%s) and amount_decimal (%s) disagree; edit amount_decimal, or remove it to use amount", shown, exact)
		}
	} else if !hasDecimal {
		amountValue = display
	}
	if !hasDecimal && !hasAmount {
		return nil, errors.New("amount is required")
	}
	amount := txSplitAmountText(amountValue)
	if !txSplitAmountRe.MatchString(amount) {
		return nil, fmt.Errorf("amount must be a plain decimal such as \"12.50\", got %v", amountValue)
	}

	split := map[string]any{"amount": amount}

	if v, ok := child["name"]; ok && v != nil {
		name, ok := v.(string)
		if !ok {
			return nil, errors.New("name must be a string")
		}
		if name != "" {
			split["name"] = name
		}
	}

	// An explicit category_id wins, null included: that clears the category.
	categoryID, hasCategoryID := child["category_id"]
	if category, ok := child["category"].(map[string]any); ok && !hasCategoryID {
		categoryID = category["id"]
	}
	if categoryID != nil {
		id, ok := categoryID.(string)
		if !ok {
			return nil, errors.New("category_id must be a string")
		}
		if id != "" {
			split["category_id"] = id
		}
	}

	if v, ok := child["excluded"]; ok && v != nil {
		excluded, ok := v.(bool)
		if !ok {
			return nil, errors.New("excluded must be true or false")
		}
		split["excluded"] = excluded
	}

	return split, nil
}

func txSplitAmountText(v any) string {
	switch v := v.(type) {
	case string:
		return v
	case json.Number:
		return v.String()
	}
	return ""
}

func txSplitSameDecimal(a, b string) bool {
	x, okX := new(big.Rat).SetString(a)
	y, okY := new(big.Rat).SetString(b)
	return okX && okY && x.Cmp(y) == 0
}

func txSplitLostFields(child map[string]any) []string {
	var lost []string
	if tags, ok := child["tags"].([]any); ok && len(tags) > 0 {
		lost = append(lost, "tags")
	}
	if notes, ok := child["notes"].(string); ok && notes != "" {
		lost = append(lost, "notes")
	}
	return lost
}
