package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/ykhoroshevskiy-tech/cell/internal/config"
	"github.com/ykhoroshevskiy-tech/cell/internal/session"
)

// completeSessionID returns flag completion entries "id\trepo (state)" for
// --session flags. Completion runs as the invoking user (__complete is in the
// read-only allowlist) and session.json files are world-readable, so this
// works after sudo as well.
func completeSessionID(cfg *config.CellConfig) func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		sm, err := session.NewSessionManager(cfg)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		list, err := sm.ListSummaries()
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		completions := make([]string, 0, len(list))
		for _, s := range list {
			state := strings.TrimSpace(s.State)
			if state == "" {
				state = "stopped"
			}
			if s.VMRunning {
				state = "running"
			}
			completions = append(completions, fmt.Sprintf("%s\t%s (%s)", s.SessionID, s.RepoSource, state))
		}
		return completions, cobra.ShellCompDirectiveNoFileComp
	}
}

func registerSessionCompletionAll(root *cobra.Command, cfg *config.CellConfig) {
	for _, sub := range root.Commands() {
		if sub.Flags().Lookup("session") != nil {
			_ = sub.RegisterFlagCompletionFunc("session", completeSessionID(cfg))
		}
	}
}
