package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/ykhoroshevskiy-tech/cell/internal/config"
	"github.com/ykhoroshevskiy-tech/cell/internal/session"
)

func newStatusCmd(cfg *config.CellConfig) *cobra.Command {
	var sessionID string
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Probe VM/SSH/tmux for one session",
		RunE: func(cmd *cobra.Command, args []string) error {
			if sessionID == "" {
				return fmt.Errorf("--session is required")
			}
			sm, err := session.NewSessionManager(cfg)
			if err != nil {
				return err
			}
			st, err := sm.Status(cmd.Context(), sessionID)
			if err != nil {
				return err
			}
			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(st)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "session=%s vm_running=%v ssh=%v tmux=%v runtime=%v guest_ip=%s tap=%s\n",
				st.SessionID, st.VMRunning, st.SSHReachable, st.TmuxReady, st.RuntimeReady, st.GuestIP, st.TapName)
			return nil
		},
	}
	cmd.Flags().StringVar(&sessionID, "session", "", "Session ID")
	cmd.Flags().BoolVar(&asJSON, "json", false, "JSON output")
	_ = cmd.MarkFlagRequired("session")
	return cmd
}
