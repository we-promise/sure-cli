package root

import (
	"fmt"
	"os"

	"github.com/dgilperez/sure-cli/internal/config"
	"github.com/spf13/cobra"
)

var (
	cfgFile string
	format  string
	quiet   bool
	trace   bool
)

func New() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sure-cli",
		Short: "Agent-first CLI for Sure (self-hosted personal finance)",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return config.Init(cfgFile)
		},
	}

	cmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default: ~/.config/sure-cli/config.yaml)")
	cmd.PersistentFlags().StringVar(&format, "format", "json", "output format: json|table")
	cmd.PersistentFlags().BoolVar(&quiet, "quiet", false, "suppress non-essential output")
	cmd.PersistentFlags().BoolVar(&trace, "trace", false, "include request tracing information")

	cmd.AddCommand(newConfigCmd())
	cmd.AddCommand(newWhoamiCmd())
	cmd.AddCommand(newAccountsCmd())
	cmd.AddCommand(newTransactionsCmd())
	cmd.AddCommand(newSyncCmd())

	return cmd
}

func Execute() {
	if err := New().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
