package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/ykhoroshevskiy-tech/cell/internal/config"
	"github.com/ykhoroshevskiy-tech/cell/internal/session"
	"github.com/ykhoroshevskiy-tech/cell/internal/ssh"
)

func newVerifyCmd(cfg *config.CellConfig) *cobra.Command {
	var sessionID string
	cmd := &cobra.Command{
		Use:   "verify",
		Short: "Readiness check with serial tail on failure",
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
			if st.RuntimeReady && st.SSHReachable && st.ServerReady {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "ok: runtime=%v ssh=%v server=%v\n", st.RuntimeReady, st.SSHReachable, st.ServerReady)
				return nil
			}
			logData, _ := sm.SerialLog(sessionID)
			tail := ssh.ReadTailFromBytes(logData, 20)
			fmt.Fprintf(os.Stderr, "verify failed: runtime=%v ssh=%v server=%v\n--- serial tail ---\n%s\n",
				st.RuntimeReady, st.SSHReachable, st.ServerReady, tail)
			return fmt.Errorf("verify failed")
		},
	}
	cmd.Flags().StringVar(&sessionID, "session", "", "Session ID")
	_ = cmd.MarkFlagRequired("session")
	return cmd
}
