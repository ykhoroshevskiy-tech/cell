package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/ykhoroshevskiy-tech/cell/internal/config"
	"github.com/ykhoroshevskiy-tech/cell/internal/session"
)

func newPsCmd(cfg *config.CellConfig) *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "ps",
		Short: "List sessions",
		RunE: func(cmd *cobra.Command, args []string) error {
			sm, err := session.NewSessionManager(cfg)
			if err != nil {
				return err
			}
			list, err := sm.List(cmd.Context(), false)
			if err != nil {
				return err
			}
			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(list)
			}
			if len(list) == 0 {
				return nil
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%-14s %-10s %-16s %-40s %s\n", "SESSION", "STATE", "GUEST_IP", "REPO", "CREATED")
			for _, s := range list {
				state := string(s.State)
				if s.VMRunning {
					state = "running"
				} else if s.State == "" {
					state = "stopped"
				}
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%-14s %-10s %-16s %-40s %s\n",
					s.SessionID, state, s.GuestIP, s.RepoSource, s.CreatedAt.Format(time.RFC3339))
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "JSON output")
	return cmd
}
