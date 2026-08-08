package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/ykhoroshevskiy-tech/cell/internal/config"
	"github.com/ykhoroshevskiy-tech/cell/internal/session"
)

func newSSHCmd(cfg *config.CellConfig) *cobra.Command {
	var sessionID string
	cmd := &cobra.Command{
		Use:   "ssh",
		Short: "Reattach SSH+tmux",
		RunE: func(cmd *cobra.Command, args []string) error {
			if sessionID == "" {
				return fmt.Errorf("--session is required")
			}
			sm, err := session.NewSessionManager(cfg)
			if err != nil {
				return err
			}
			return sm.Attach(cmd.Context(), sessionID)
		},
	}
	cmd.Flags().StringVar(&sessionID, "session", "", "Session ID")
	_ = cmd.MarkFlagRequired("session")
	return cmd
}
