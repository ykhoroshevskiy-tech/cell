package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/ykhoroshevskiy-tech/cell/internal/config"
	"github.com/ykhoroshevskiy-tech/cell/internal/session"
)

func newLogsCmd(cfg *config.CellConfig) *cobra.Command {
	var sessionID string
	cmd := &cobra.Command{
		Use:   "logs",
		Short: "Print serial.log",
		RunE: func(cmd *cobra.Command, args []string) error {
			if sessionID == "" {
				return fmt.Errorf("--session is required")
			}
			sm, err := session.NewSessionManager(cfg)
			if err != nil {
				return err
			}
			data, err := sm.SerialLog(sessionID)
			if err != nil {
				return err
			}
			_, err = os.Stdout.Write(data)
			return err
		},
	}
	cmd.Flags().StringVar(&sessionID, "session", "", "Session ID")
	_ = cmd.MarkFlagRequired("session")
	return cmd
}
