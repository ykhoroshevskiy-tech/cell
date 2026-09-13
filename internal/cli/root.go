package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/ykhoroshevskiy-tech/cell/internal/config"
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
	root.AddCommand(newVersionCmd())
	return root
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version",
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.Println("cell 0.1.0")
			return nil
		},
	}
}
