package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/ykhoroshevskiy-tech/cell/internal/config"
	"github.com/ykhoroshevskiy-tech/cell/internal/models"
	"github.com/ykhoroshevskiy-tech/cell/internal/session"
)

func newPullCmd(cfg *config.CellConfig) *cobra.Command {
	var sessionID, dest string
	var dryRun, deleteFiles bool
	cmd := &cobra.Command{
		Use:   "pull",
		Short: "Rsync guest workspace back to host repo",
		RunE: func(cmd *cobra.Command, args []string) error {
			if sessionID == "" {
				return fmt.Errorf("--session is required")
			}
			sm, err := session.NewSessionManager(cfg)
			if err != nil {
				return err
			}
			result, err := sm.Pull(cmd.Context(), sessionID, models.PullOptions{
				Dest:   dest,
				DryRun: dryRun,
				Delete: deleteFiles,
			})
			if err != nil {
				return err
			}
			fmt.Printf("pulled to %s\n", result.Destination)
			return nil
		},
	}
	cmd.Flags().StringVar(&sessionID, "session", "", "Session ID")
	cmd.Flags().StringVar(&dest, "dest", "", "Destination path (default: session repo source)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show planned rsync changes only")
	cmd.Flags().BoolVar(&deleteFiles, "delete", false, "Delete host files absent in guest")
	_ = cmd.MarkFlagRequired("session")
	return cmd
}
