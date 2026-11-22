package root

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/we-promise/sure-cli/internal/config"
	errs "github.com/we-promise/sure-cli/internal/errors"
)

var (
	cfgFile string
	format  string

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
		// The config is validated by the API client (api.New), so commands
		// that never call Sure run with an invalid one.
		// An unreadable config fails here as validation_failed; returned
		// to cobra, it would also print usage as if the command were wrong.
		PersistentPreRun: func(cmd *cobra.Command, args []string) { failValidation(config.Init(cfgFile)) },
	}

	cmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default: ~/.config/sure-cli/config.yaml)")
	cmd.PersistentFlags().StringVar(&format, "format", "json", "output format: json|table (table for list commands that have one; others print JSON)")
	cmd.PersistentFlags().String("timeout", config.DefaultTimeout.String(), "limit for one API call, retries included; for downloads, for each pause in the transfer: a duration (90s, 2m) or seconds (0 disables)")
	_ = viper.BindPFlag("timeout", cmd.PersistentFlags().Lookup("timeout"))
	cmd.PersistentFlags().Bool("allow-insecure-http", false, "allow a plain http:// api_url on a host other than this machine (sends credentials unencrypted)")
	_ = viper.BindPFlag(config.AllowInsecureHTTP, cmd.PersistentFlags().Lookup("allow-insecure-http"))

	cmd.AddCommand(newConfigCmd())
	cmd.AddCommand(newLoginCmd())
	cmd.AddCommand(newRefreshCmd())
	cmd.AddCommand(newWhoamiCmd())
	cmd.AddCommand(newAccountsCmd())
	cmd.AddCommand(newCategoriesCmd())
	cmd.AddCommand(newMerchantsCmd())
	cmd.AddCommand(newTagsCmd())
	cmd.AddCommand(newTransactionsCmd())
	cmd.AddCommand(newImportsCmd())
	cmd.AddCommand(newImportSessionsCmd())
	cmd.AddCommand(newAccountStatementsCmd())
	cmd.AddCommand(newFamilyExportsCmd())
	cmd.AddCommand(newRulesCmd())
	cmd.AddCommand(newRuleRunsCmd())
	cmd.AddCommand(newBalanceSheetCmd())
	cmd.AddCommand(newCashFlowCmd())
	cmd.AddCommand(newSpendingVsNormalCmd())
	cmd.AddCommand(newBalancesCmd())
	cmd.AddCommand(newBudgetsCmd())
	cmd.AddCommand(newBudgetCategoriesCmd())
	cmd.AddCommand(newFamilySettingsCmd())
	cmd.AddCommand(newValuationsCmd())
	cmd.AddCommand(newSyncCmd())
	cmd.AddCommand(newSyncsCmd())
	cmd.AddCommand(newUsageCmd())
	cmd.AddCommand(newInsightsCmd())
	cmd.AddCommand(newAnalyzeCmd())
	cmd.AddCommand(newProposeCmd())
	cmd.AddCommand(newExportCmd())
	cmd.AddCommand(newStatusCmd())
	cmd.AddCommand(newHoldingsCmd())
	cmd.AddCommand(newSecuritiesCmd())
	cmd.AddCommand(newSecurityPricesCmd())
	cmd.AddCommand(newTradesCmd())
	cmd.AddCommand(newRecurringTransactionsCmd())
	cmd.AddCommand(newUsersCmd())
	cmd.AddCommand(newTransfersCmd())
	cmd.AddCommand(newRejectedTransfersCmd())
	cmd.AddCommand(newProviderConnectionsCmd())
	cmd.AddCommand(newChatsCmd())
	cmd.AddCommand(newAuthCmd())
	cmd.AddCommand(newVersionCmd())

	// cobra adds completion when it executes; add it now so it is guarded.
	cmd.InitDefaultCompletionCmd()
	guardCommandGroups(cmd)
	return cmd
}

// removedCommands points the command paths earlier releases had to what
// replaces them.
var removedCommands = map[string]string{
	"insights subscriptions": "use `status` (data.monthly.subscriptions) or Sure's subscription insights in `insights list`",
	"insights fees":          "use `insights list` (Sure's bank fee insights)",
	"insights leaks":         "use `analyze leaks`",
	"plan":                   "use `analyze runway`, `budgets list` or `insights list`",
	"plan runway":            "use `analyze runway`",
	"plan budget":            "use `budgets list` and `budget-categories list`",
	"plan forecast":          "use `insights list` (Sure's cash flow insights)",
	"config heuristics":      "leak thresholds are `analyze leaks` flags; Sure's `insights list` covers subscriptions and fees",
	"config fee-keywords":    "Sure detects bank fees itself, see `insights list`",
}

// guardCommandGroups makes every command without Args refuse arguments, and
// every command group (root included) fail with an unknown_command envelope
// when given a subcommand it does not have, instead of printing help and
// exiting 0. A group run bare still prints its help.
func guardCommandGroups(cmd *cobra.Command) {
	runnable, group := cmd.Runnable(), cmd.HasSubCommands()
	switch {
	case runnable && !group && cmd.Args == nil:
		// A command that names no Args would take stray arguments and ignore
		// them (`export transactions out.csv` wrote the default file).
		cmd.Args = cobra.NoArgs
	case runnable && group:
		// A group that also runs on its own (users reset) sees an unknown
		// subcommand as an argument: report it as one, not as bad arguments.
		takes := cmd.Args
		cmd.Args = func(c *cobra.Command, args []string) error {
			if len(args) > 0 && (takes == nil || takes(c, args) != nil) {
				failUnknownCommand(c, args)
			}
			return nil
		}
	case group:
		cmd.Args = cobra.ArbitraryArgs
		cmd.SuggestionsMinimumDistance = 2 // cobra's default, which SuggestionsFor does not apply
		cmd.RunE = func(c *cobra.Command, args []string) error {
			if len(args) == 0 {
				return c.Help()
			}
			failUnknownCommand(c, args)
			return nil
		}
		// A removed command's flags fail to parse on its group before RunE
		// runs; the words parsed so far still name the command. Commands under
		// the group inherit this func: their flag errors are about the flag.
		cmd.SetFlagErrorFunc(func(c *cobra.Command, err error) error {
			if c != cmd {
				failValidation(err)
			}
			if args := c.Flags().Args(); len(args) > 0 {
				failUnknownCommand(c, args)
			}
			return err
		})
	}
	for _, sub := range cmd.Commands() {
		guardCommandGroups(sub)
	}
}

func failUnknownCommand(c *cobra.Command, args []string) {
	fail(unknownCommandError(c, args))
}

// unknownCommandError describes args[0] as an unknown subcommand of c, with
// cobra's suggestions and, for a removed command, its replacement.
func unknownCommandError(c *cobra.Command, args []string) *errs.CLIError {
	group := strings.TrimPrefix(c.CommandPath(), c.Root().Name())
	named := func(words []string) string { return strings.TrimSpace(group + " " + strings.Join(words, " ")) }
	details := map[string]any{"command": named(args[:1])}
	if s := c.SuggestionsFor(args[0]); len(s) > 0 {
		details["suggestions"] = s
	}
	msg := fmt.Sprintf("unknown command %q for %q", args[0], c.CommandPath())
	for n := min(2, len(args)); n > 0; n-- {
		removed := named(args[:n])
		if hint, ok := removedCommands[removed]; ok {
			details["command"], details["hint"] = removed, hint
			msg = fmt.Sprintf("command %q was removed; %s", removed, hint)
			break
		}
	}
	return errs.New(errs.CodeUnknownCommand, msg).WithDetails(details)
}

func Execute() { run(New()) }

// run executes cli. An error cobra returns is a usage error caught before
// any command ran (an unknown flag, a wrong number of arguments): it fails
// with a validation_failed envelope like every input the CLI rejects.
func run(cli *cobra.Command) {
	cli.SilenceErrors = true // failValidation prints the message once
	cli.SilenceUsage = true  // the envelope names the problem; --help shows usage
	if err := cli.Execute(); err != nil {
		failValidation(err)
	}
}
