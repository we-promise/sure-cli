package root

import (
	"github.com/dgilperez/sure-cli/internal/config"
	"github.com/dgilperez/sure-cli/internal/insights"
	"github.com/dgilperez/sure-cli/internal/output"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "config", Short: "Manage configuration"}
	cmd.AddCommand(&cobra.Command{
		Use:   "get <key>",
		Short: "Get a config value",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			val := viper.Get(args[0])
			_ = output.Print(format, output.Envelope{Data: map[string]any{"key": args[0], "value": val}})
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a config value",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			viper.Set(args[0], args[1])
			if err := config.Save(); err != nil {
				output.Fail("config_save_failed", err.Error(), nil)
			}
			_ = output.Print(format, output.Envelope{Data: map[string]any{"ok": true}})
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "heuristics",
		Short: "Show current heuristics configuration",
		Run: func(cmd *cobra.Command, args []string) {
			h := config.GetHeuristics()
			// If no custom keywords, show that defaults will be used
			keywordsInfo := map[string]any{
				"custom":        h.Fees.Keywords,
				"using_default": len(h.Fees.Keywords) == 0,
			}
			if len(h.Fees.Keywords) == 0 {
				keywordsInfo["default_count"] = len(insights.DefaultFeeKeywords)
			}
			_ = output.Print(format, output.Envelope{Data: map[string]any{
				"fees": map[string]any{
					"keywords": keywordsInfo,
				},
				"subscriptions": h.Subscriptions,
				"leaks":         h.Leaks,
				"rules":         h.Rules,
			}})
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "fee-keywords",
		Short: "Show all default fee keywords",
		Run: func(cmd *cobra.Command, args []string) {
			custom := config.GetFeeKeywords()
			active := insights.GetFeeKeywords(custom)
			_ = output.Print(format, output.Envelope{Data: map[string]any{
				"active_keywords": active,
				"count":           len(active),
				"is_custom":       len(custom) > 0,
			}})
		},
	})
	return cmd
}
