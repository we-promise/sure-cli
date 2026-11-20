package root

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/we-promise/sure-cli/internal/config"
	errs "github.com/we-promise/sure-cli/internal/errors"
	"github.com/we-promise/sure-cli/internal/output"
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
			config.Set(args[0], args[1])
			if err := config.Save(); err != nil {
				output.Fail(errs.CodeConfigSaveFailed, err.Error(), nil)
			}
			_ = output.Print(format, output.Envelope{Data: map[string]any{"ok": true}})
		},
	})
	return cmd
}
