package root

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/we-promise/sure-cli/internal/config"
	errs "github.com/we-promise/sure-cli/internal/errors"
	"github.com/we-promise/sure-cli/internal/output"
)

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage configuration",
		// Skips the root's Validate: these commands must work to repair an
		// invalid setting.
		PersistentPreRunE: func(*cobra.Command, []string) error { return config.Init(cfgFile) },
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "get <key>",
		Short: "Get a config value",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			val := viper.Get(args[0])
			printEnvelope(map[string]any{"key": args[0], "value": val}, nil)
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a config value",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			if args[0] == "timeout" {
				_, err := config.ParseTimeout(args[1])
				failValidation(err)
			}
			config.Set(args[0], args[1])
			if err := config.Save(); err != nil {
				output.Fail(errs.CodeConfigSaveFailed, err.Error(), nil)
			}
			printEnvelope(map[string]any{"ok": true}, nil)
		},
	})
	return cmd
}
