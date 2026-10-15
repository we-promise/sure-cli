package root

import (
	"fmt"
	"net/url"
	"os"

	"github.com/spf13/cobra"
)

type transactionSplitOpts struct {
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
			printGet(transactionSplitPath(args[0]))
		},
	})

	cmd.AddCommand(newTransactionsSplitWriteCmd(
		"create <transaction_id>",
		"Split a transaction (default dry-run; use --apply to execute)",
		"POST",
	))
	cmd.AddCommand(newTransactionsSplitWriteCmd(
		"update <transaction_id>",
		"Replace every child of a split (default dry-run; use --apply to execute)",
		"PATCH",
	))

	var apply bool
	deleteCmd := &cobra.Command{
		Use:   "delete <transaction_id>",
		Short: "Remove a split and restore the parent (default dry-run; use --apply to execute)",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			dispatchWrite(apply, "DELETE", transactionSplitPath(args[0]), nil)
		},
	}
	deleteCmd.Flags().BoolVar(&apply, "apply", false, "execute the delete (otherwise dry-run)")
	cmd.AddCommand(deleteCmd)

	return cmd
}

func newTransactionsSplitWriteCmd(use, short, method string) *cobra.Command {
	var o transactionSplitOpts

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
			raw, err := readTransactionSplitInput(o, os.Stdin)
			if err != nil {
				failValidation(err)
				return
			}
			body, err := buildTransactionSplitPayload(raw)
			if err != nil {
				failValidation(err)
				return
			}
			dispatchWrite(o.Apply, method, transactionSplitPath(args[0]), body)
		},
	}

	cmd.Flags().StringVar(&o.Splits, "splits", "", "children as a JSON array")
	cmd.Flags().StringVar(&o.SplitsFile, "splits-file", "", "path to a JSON file of children (- for stdin)")
	cmd.Flags().BoolVar(&o.Apply, "apply", false, "execute the write (otherwise dry-run)")
	return cmd
}

func transactionSplitPath(id string) string {
	return fmt.Sprintf("/api/v1/transactions/%s/split", url.PathEscape(id))
}
