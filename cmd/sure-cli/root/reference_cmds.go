package root

import (
	"fmt"
	"net/url"

	"github.com/spf13/cobra"
	"github.com/we-promise/sure-cli/internal/output"
)

func newCategoriesCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "categories", Short: "Categories"}

	var page, perPage int
	var rootsOnly bool
	var parentID string
	list := &cobra.Command{
		Use:   "list",
		Short: "List categories",
		Run: func(cmd *cobra.Command, args []string) {
			q := url.Values{}
			addPagingQuery(q, page, perPage)
			if rootsOnly {
				q.Set("roots_only", "true")
			}
			if parentID != "" {
				q.Set("parent_id", parentID)
			}
			printGet(pathWithQuery("/api/v1/categories", q))
		},
	}
	addPagingFlags(list, &page, &perPage)
	list.Flags().BoolVar(&rootsOnly, "roots-only", false, "return root categories only")
	list.Flags().StringVar(&parentID, "parent-id", "", "filter by parent category id")
	cmd.AddCommand(list)

	cmd.AddCommand(&cobra.Command{
		Use:   "show <id>",
		Short: "Show category",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			printGet(fmt.Sprintf("/api/v1/categories/%s", args[0]))
		},
	})

	return cmd
}

func newMerchantsCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "merchants", Short: "Merchants"}
	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List merchants",
		Run: func(cmd *cobra.Command, args []string) {
			printGet("/api/v1/merchants")
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "show <id>",
		Short: "Show merchant",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			printGet(fmt.Sprintf("/api/v1/merchants/%s", args[0]))
		},
	})
	return cmd
}

type tagWriteOpts struct {
	Name  string
	Color string
	Apply bool
}

func newTagsCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "tags", Short: "Tags"}
	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List tags",
		Run: func(cmd *cobra.Command, args []string) {
			printGet("/api/v1/tags")
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "show <id>",
		Short: "Show tag",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			printGet(fmt.Sprintf("/api/v1/tags/%s", args[0]))
		},
	})
	cmd.AddCommand(newTagsCreateCmd())
	cmd.AddCommand(newTagsUpdateCmd())
	cmd.AddCommand(newTagsDeleteCmd())
	return cmd
}

func newTagsCreateCmd() *cobra.Command {
	var o tagWriteOpts
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create tag (default dry-run; use --apply to execute)",
		Run: func(cmd *cobra.Command, args []string) {
			payload, err := buildTagPayload(o, true)
			if err != nil {
				failValidation(err)
			}
			path := "/api/v1/tags"
			if !o.Apply {
				printDryRun("POST", path, payload)
				return
			}
			printPost(path, payload)
		},
	}
	cmd.Flags().StringVar(&o.Name, "name", "", "tag name (required)")
	cmd.Flags().StringVar(&o.Color, "color", "", "hex color")
	cmd.Flags().BoolVar(&o.Apply, "apply", false, "execute the create (otherwise dry-run)")
	return cmd
}

func newTagsUpdateCmd() *cobra.Command {
	var o tagWriteOpts
	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update tag (default dry-run; use --apply to execute)",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			payload, err := buildTagPayload(o, false)
			if err != nil {
				failValidation(err)
			}
			path := fmt.Sprintf("/api/v1/tags/%s", args[0])
			if !o.Apply {
				printDryRun("PATCH", path, payload)
				return
			}
			printPatch(path, payload)
		},
	}
	cmd.Flags().StringVar(&o.Name, "name", "", "tag name")
	cmd.Flags().StringVar(&o.Color, "color", "", "hex color")
	cmd.Flags().BoolVar(&o.Apply, "apply", false, "execute the update (otherwise dry-run)")
	return cmd
}

func newTagsDeleteCmd() *cobra.Command {
	var apply bool
	cmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete tag (default dry-run; use --apply to execute)",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			path := fmt.Sprintf("/api/v1/tags/%s", args[0])
			if !apply {
				printDryRun("DELETE", path, nil)
				return
			}
			printDelete(path)
		},
	}
	cmd.Flags().BoolVar(&apply, "apply", false, "execute the delete (otherwise dry-run)")
	return cmd
}

func buildTagPayload(o tagWriteOpts, requireName bool) (map[string]any, error) {
	if requireName && o.Name == "" {
		return nil, fmt.Errorf("name is required")
	}
	tag := map[string]any{}
	if o.Name != "" {
		tag["name"] = o.Name
	}
	if o.Color != "" {
		tag["color"] = o.Color
	}
	if len(tag) == 0 {
		return nil, fmt.Errorf("no fields provided")
	}
	return map[string]any{"tag": tag}, nil
}

func newRulesCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "rules", Short: "Rules"}

	var page, perPage int
	var resourceType, active string
	list := &cobra.Command{
		Use:   "list",
		Short: "List rules",
		Run: func(cmd *cobra.Command, args []string) {
			q := url.Values{}
			addPagingQuery(q, page, perPage)
			if resourceType != "" {
				q.Set("resource_type", resourceType)
			}
			if active != "" {
				q.Set("active", active)
			}
			printGet(pathWithQuery("/api/v1/rules", q))
		},
	}
	addPagingFlags(list, &page, &perPage)
	list.Flags().StringVar(&resourceType, "resource-type", "", "resource type filter")
	list.Flags().StringVar(&active, "active", "", "active filter: true|false|1|0")
	cmd.AddCommand(list)

	cmd.AddCommand(&cobra.Command{
		Use:   "show <id>",
		Short: "Show rule",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			printGet(fmt.Sprintf("/api/v1/rules/%s", args[0]))
		},
	})

	return cmd
}

func newRuleRunsCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "rule-runs", Short: "Rule runs"}

	var page, perPage int
	var ruleID, status, executionType, startExecutedAt, endExecutedAt string
	list := &cobra.Command{
		Use:   "list",
		Short: "List rule runs",
		Run: func(cmd *cobra.Command, args []string) {
			q := url.Values{}
			addPagingQuery(q, page, perPage)
			if ruleID != "" {
				q.Set("rule_id", ruleID)
			}
			if status != "" {
				q.Set("status", status)
			}
			if executionType != "" {
				q.Set("execution_type", executionType)
			}
			if startExecutedAt != "" {
				q.Set("start_executed_at", startExecutedAt)
			}
			if endExecutedAt != "" {
				q.Set("end_executed_at", endExecutedAt)
			}
			printGet(pathWithQuery("/api/v1/rule_runs", q))
		},
	}
	addPagingFlags(list, &page, &perPage)
	list.Flags().StringVar(&ruleID, "rule-id", "", "rule id filter")
	list.Flags().StringVar(&status, "status", "", "status filter: pending|success|failed")
	list.Flags().StringVar(&executionType, "execution-type", "", "execution type filter: manual|scheduled")
	list.Flags().StringVar(&startExecutedAt, "start-executed-at", "", "start executed timestamp (ISO 8601)")
	list.Flags().StringVar(&endExecutedAt, "end-executed-at", "", "end executed timestamp (ISO 8601)")
	cmd.AddCommand(list)

	cmd.AddCommand(&cobra.Command{
		Use:   "show <id>",
		Short: "Show rule run",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			printGet(fmt.Sprintf("/api/v1/rule_runs/%s", args[0]))
		},
	})

	return cmd
}

func failValidation(err error) {
	if err != nil {
		output.Fail("validation_failed", err.Error(), nil)
	}
}
