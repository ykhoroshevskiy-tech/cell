package cli

import (
	"github.com/spf13/cobra"
	"github.com/ykhoroshevskiy-tech/cell/internal/config"
	"github.com/ykhoroshevskiy-tech/cell/internal/verbose"
)

var quiet bool
var verboseFlag bool

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
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			verbose.Verbose = verboseFlag
		},
	}
	root.PersistentFlags().BoolVar(&quiet, "quiet", false, "Suppress non-error output")
	root.PersistentFlags().BoolVarP(&verboseFlag, "verbose", "v", false, "Verbose output (mkfs, fc requests, ssh polling, etc.)")

	root.AddCommand(newBootstrapCmd(cfg))
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
