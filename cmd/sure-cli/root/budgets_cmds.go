package root

import (
	"fmt"
	"net/url"

	"github.com/spf13/cobra"
)

func newBudgetsCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "budgets", Short: "Budgets"}

	var page, perPage int
	var startDate, endDate string
	list := &cobra.Command{
		Use:   "list",
		Short: "List budgets",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			q := url.Values{}
			addPagingQuery(q, page, perPage)
			if startDate != "" {
				q.Set("start_date", startDate)
			}
			if endDate != "" {
				q.Set("end_date", endDate)
			}
			printGet(pathWithQuery("/api/v1/budgets", q))
		},
	}
	addPagingFlags(list, &page, &perPage)
	list.Flags().StringVar(&startDate, "start-date", "", "filter budgets with start_date >= (YYYY-MM-DD)")
	list.Flags().StringVar(&endDate, "end-date", "", "filter budgets with end_date <= (YYYY-MM-DD)")
	cmd.AddCommand(list)

	cmd.AddCommand(&cobra.Command{
		Use:   "show <id>",
		Short: "Show budget",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			printGet(fmt.Sprintf("/api/v1/budgets/%s", url.PathEscape(args[0])))
		},
	})

	return cmd
}

func newBudgetCategoriesCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "budget-categories", Short: "Budget categories"}

	var page, perPage int
	var budgetID, categoryID, startDate, endDate string
	list := &cobra.Command{
		Use:   "list",
		Short: "List budget categories",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			q := url.Values{}
			addPagingQuery(q, page, perPage)
			if budgetID != "" {
				q.Set("budget_id", budgetID)
			}
			if categoryID != "" {
				q.Set("category_id", categoryID)
			}
			if startDate != "" {
				q.Set("start_date", startDate)
			}
			if endDate != "" {
				q.Set("end_date", endDate)
			}
			printGet(pathWithQuery("/api/v1/budget_categories", q))
		},
	}
	addPagingFlags(list, &page, &perPage)
	list.Flags().StringVar(&budgetID, "budget-id", "", "filter by budget id (UUID)")
	list.Flags().StringVar(&categoryID, "category-id", "", "filter by category id (UUID)")
	list.Flags().StringVar(&startDate, "start-date", "", "filter budgets with start_date >= (YYYY-MM-DD)")
	list.Flags().StringVar(&endDate, "end-date", "", "filter budgets with end_date <= (YYYY-MM-DD)")
	cmd.AddCommand(list)

	cmd.AddCommand(&cobra.Command{
		Use:   "show <id>",
		Short: "Show budget category",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			printGet(fmt.Sprintf("/api/v1/budget_categories/%s", url.PathEscape(args[0])))
		},
	})

	return cmd
}
