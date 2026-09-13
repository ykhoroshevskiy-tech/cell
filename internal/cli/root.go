package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/ykhoroshevskiy-tech/cell/internal/config"
	"github.com/ykhoroshevskiy-tech/cell/internal/version"
	"github.com/ykhoroshevskiy-tech/cell/internal/verbose"
)

var quiet bool
var verboseFlag bool

// readOnlyCommands only read world-readable session state and never touch
// the network, disks, or VM processes; they run without root.
var readOnlyCommands = map[string]struct{}{
	"ps":               {},
	"logs":             {},
	"version":          {},
	"help":             {},
	"completion":       {},
	"__complete":       {},
	"__completeNoDesc": {},
}

func Execute() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	return newRootCmd(cfg).Execute()
}

func newRootCmd(cfg *config.CellConfig) *cobra.Command {
	root := &cobra.Command{
		Use:           "cell",
		Short:         "Coding-agent microVM runtime manager",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			verbose.Verbose = verboseFlag
			if _, ok := readOnlyCommands[cmd.Name()]; ok {
				return nil
			}
			if os.Geteuid() == 0 {
				return nil
			}
			return fmt.Errorf("cell: %s requires root — run: sudo cell %s", cmd.Name(), cmd.Name())
		},
	}
	root.PersistentFlags().BoolVar(&quiet, "quiet", false, "Suppress non-error output")
	root.PersistentFlags().BoolVarP(&verboseFlag, "verbose", "v", false, "Verbose output (mkfs, fc requests, ssh polling, etc.)")

	root.AddCommand(newBootstrapCmd(cfg))
	root.AddCommand(newNetSetupCmd())
	root.AddCommand(newLaunchCmd(cfg))
	root.AddCommand(newStartCmd(cfg))
	root.AddCommand(newAttachCmd(cfg))
	root.AddCommand(newStopCmd(cfg))
	root.AddCommand(newSSHCmd(cfg))
	root.AddCommand(newStatusCmd(cfg))
	root.AddCommand(newVerifyCmd(cfg))
	root.AddCommand(newLogsCmd(cfg))
	root.AddCommand(newPsCmd(cfg))
	root.AddCommand(newPullCmd(cfg))
	root.AddCommand(newRescueCmd(cfg))
	root.AddCommand(newRmCmd(cfg))
	root.AddCommand(newCompletionCmd())
	root.AddCommand(newVersionCmd())
	registerSessionCompletionAll(root, cfg)
	return root
}

func newCompletionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "completion [bash|zsh|fish|powershell]",
		Short: "Generate shell completion script",
		Long: `Generate shell completion script.

Load completions in the current shell (zsh):
  source <(cell completion zsh)

Install permanently (zsh):
  cell completion zsh > "${fpath[1]}/_cell"`,
		Args:      cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		ValidArgs: []string{"bash", "zsh", "fish", "powershell"},
		RunE: func(cmd *cobra.Command, args []string) error {
			switch args[0] {
			case "bash":
				return cmd.Root().GenBashCompletionV2(cmd.OutOrStdout(), true)
			case "zsh":
				return cmd.Root().GenZshCompletion(cmd.OutOrStdout())
			case "fish":
				return cmd.Root().GenFishCompletion(cmd.OutOrStdout(), true)
			case "powershell":
				return cmd.Root().GenPowerShellCompletionWithDesc(cmd.OutOrStdout())
			}
			return nil
		},
	}
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version",
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.Printf("cell %s\n", version.Version)
			return nil
		},
	}
}
