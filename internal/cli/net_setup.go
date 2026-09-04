package cli

import (
	"github.com/spf13/cobra"
	"github.com/ykhoroshevskiy-tech/cell/internal/network"
)

func newNetSetupCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "net-setup",
		Hidden: true,
		Short:  "Ensure bridge, firewall, and network lock",
		RunE: func(cmd *cobra.Command, args []string) error {
			return network.NetSetupFull()
		},
	}
}
