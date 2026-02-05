package root

import (
	"fmt"
	"os"

	"github.com/we-promise/sure-cli/internal/config"
	"github.com/spf13/cobra"
)

var (
	cfgFile string
	format  string
	quiet   bool
	trace   bool

	// Version info (set by main via SetVersion)
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

// SetVersion sets version info from main (populated by goreleaser ldflags)
func SetVersion(v, c, d string) {
	version = v
	commit = c
	date = d
}

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
	cmd.AddCommand(newLoginCmd())
	cmd.AddCommand(newRefreshCmd())
	cmd.AddCommand(newWhoamiCmd())
	cmd.AddCommand(newAccountsCmd())
	cmd.AddCommand(newTransactionsCmd())
	cmd.AddCommand(newSyncCmd())
	cmd.AddCommand(newInsightsCmd())
	cmd.AddCommand(newPlanCmd())
	cmd.AddCommand(newProposeCmd())
	cmd.AddCommand(newExportCmd())
	cmd.AddCommand(newStatusCmd())
	cmd.AddCommand(newHoldingsCmd())
	cmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("sure-cli %s\n", version)
			fmt.Printf("  commit: %s\n", commit)
			fmt.Printf("  built:  %s\n", date)
		},
	})

	return cmd
}

func Execute() {
	if err := New().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
