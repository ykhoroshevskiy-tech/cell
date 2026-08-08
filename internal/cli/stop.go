package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/ykhoroshevskiy-tech/cell/internal/config"
	"github.com/ykhoroshevskiy-tech/cell/internal/session"
)

func newStopCmd(cfg *config.CellConfig) *cobra.Command {
	var sessionID string
	var all bool
	cmd := &cobra.Command{
		Use:   "stop",
		Short: "Stop one or all VMs",
		RunE: func(cmd *cobra.Command, args []string) error {
			if (sessionID == "" && !all) || (sessionID != "" && all) {
				return fmt.Errorf("specify exactly one of --session or --all")
			}
			sm, err := session.NewSessionManager(cfg)
			if err != nil {
				return err
			}
			if all {
				n, errs := sm.StopAll(cmd.Context())
				fmt.Printf("stopped %d session(s)\n", n)
				if len(errs) > 0 {
					for _, e := range errs {
						fmt.Fprintf(os.Stderr, "stop error: %v\n", e)
					}
					return fmt.Errorf("some sessions failed to stop")
				}
				return nil
			}
			if err := sm.Stop(cmd.Context(), sessionID); err != nil {
				return err
			}
			fmt.Printf("stopped %s\n", sessionID)
			return nil
		},
	}
	cmd.Flags().StringVar(&sessionID, "session", "", "Session ID")
	cmd.Flags().BoolVar(&all, "all", false, "Stop all running sessions")
	return cmd
}
