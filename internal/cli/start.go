package cli

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/ykhoroshevskiy-tech/cell/internal/config"
	"github.com/ykhoroshevskiy-tech/cell/internal/session"
)

func newStartCmd(cfg *config.CellConfig) *cobra.Command {
	var sessionID string
	var noAttach bool
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Boot an existing session disk",
		RunE: func(cmd *cobra.Command, args []string) error {
			if sessionID == "" {
				return fmt.Errorf("--session is required")
			}
			sm, err := session.NewSessionManager(cfg)
			if err != nil {
				return err
			}
			err = sm.Start(cmd.Context(), sessionID, !noAttach)
			if errors.Is(err, session.ErrAlreadyRunning) {
				fmt.Printf("session %s already running; cell attach --session %s\n", sessionID, sessionID)
				return nil
			}
			return err
		},
	}
	cmd.Flags().StringVar(&sessionID, "session", "", "Session ID")
	cmd.Flags().BoolVar(&noAttach, "no-attach", false, "Wait for ready but do not exec SSH")
	_ = cmd.MarkFlagRequired("session")
	return cmd
}
