package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/ykhoroshevskiy-tech/cell/internal/config"
	"github.com/ykhoroshevskiy-tech/cell/internal/session"
)

func newRmCmd(cfg *config.CellConfig) *cobra.Command {
	var sessionID string
	var legacy bool
	cmd := &cobra.Command{
		Use:   "rm",
		Short: "Remove a stopped session and free its network lease",
		RunE: func(cmd *cobra.Command, args []string) error {
			if sessionID == "" {
				return fmt.Errorf("--session required")
			}
			sm, err := session.NewSessionManager(cfg)
			if err != nil {
				return err
			}
			if legacy {
				return sm.RemoveLegacy(cmd.Context(), sessionID)
			}
			return sm.Remove(cmd.Context(), sessionID)
		},
	}
	cmd.Flags().StringVar(&sessionID, "session", "", "Session ID")
	cmd.Flags().BoolVar(&legacy, "legacy", false, "Remove legacy (pre-bridge) session records")
	return cmd
}
