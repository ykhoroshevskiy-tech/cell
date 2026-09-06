package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/ykhoroshevskiy-tech/cell/internal/config"
	"github.com/ykhoroshevskiy-tech/cell/internal/privilege"
	"github.com/ykhoroshevskiy-tech/cell/internal/session"
)

func newRescueCmd(cfg *config.CellConfig) *cobra.Command {
	var sessionID, dest string
	cmd := &cobra.Command{
		Use:   "rescue",
		Short: "Extract workspace from project disk",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := privilege.MaybeElevate("rescue"); err != nil {
				return err
			}
			if sessionID == "" || dest == "" {
				return fmt.Errorf("--session and --dest are required")
			}
			sm, err := session.NewSessionManager(cfg)
			if err != nil {
				return err
			}
			if err := sm.Rescue(cmd.Context(), sessionID, dest); err != nil {
				return err
			}
			fmt.Printf("rescued session %s to %s\n", sessionID, dest)
			return nil
		},
	}
	cmd.Flags().StringVar(&sessionID, "session", "", "Session ID")
	cmd.Flags().StringVar(&dest, "dest", "", "Destination directory")
	_ = cmd.MarkFlagRequired("session")
	_ = cmd.MarkFlagRequired("dest")
	return cmd
}
