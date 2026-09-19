package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/ykhoroshevskiy-tech/cell/internal/bootstrap"
	"github.com/ykhoroshevskiy-tech/cell/internal/config"
	"github.com/ykhoroshevskiy-tech/cell/internal/models"
	"github.com/ykhoroshevskiy-tech/cell/internal/session"
	"github.com/ykhoroshevskiy-tech/cell/internal/verbose"
)

func newBootstrapCmd(cfg *config.CellConfig) *cobra.Command {
	var force, rebuildRootfs bool
	cmd := &cobra.Command{
		Use:   "bootstrap",
		Short: "Download/build kernel, rootfs, firecracker",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := bootstrap.Ensure(cfg, force, rebuildRootfs); err != nil {
				return err
			}
			fmt.Printf("firecracker: %s\n", cfg.FirecrackerBin)
			fmt.Printf("kernel:      %s\n", cfg.KernelPath)
			fmt.Printf("rootfs:      %s\n", cfg.RootfsPath)
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "Re-download kernel, firecracker; rebuild rootfs")
	cmd.Flags().BoolVar(&rebuildRootfs, "rebuild-rootfs", false, "Rebuild rootfs only")
	return cmd
}

func newLaunchCmd(cfg *config.CellConfig) *cobra.Command {
	var repo string
	var noAttach bool
	var configPath string
	var agentKind string
	cmd := &cobra.Command{
		Use:   "launch",
		Short: "Stage repo, boot VM, attach SSH, auto-pull",
		RunE: func(cmd *cobra.Command, args []string) error {
			if repo == "" {
				return fmt.Errorf("--repo is required")
			}
			kind := models.NormalizeAgentKind(agentKind)
			if kind == "" {
				return fmt.Errorf("invalid --agent %q (want opencode|claude|none)", agentKind)
			}
			verbose.V("cell launch repo=%s attach=%v agent=%s", repo, !noAttach, kind)
			if err := os.MkdirAll(cfg.SessionDataDir, 0755); err != nil {
				return err
			}
			if err := bootstrap.Ensure(cfg, false, false); err != nil {
				return fmt.Errorf("bootstrap: %w", err)
			}
			sm, err := session.NewSessionManager(cfg)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
			go func() {
				<-sigCh
				cancel()
			}()

			agentConfig := ""
			if configPath != "" {
				abs, warn, err := session.ResolveAgentConfig(configPath)
				if err != nil {
					return err
				}
				if warn != "" {
					_, _ = fmt.Fprintln(cmd.ErrOrStderr(), warn)
				}
				agentConfig = abs
			}

			sess, err := sm.Launch(ctx, repo, agentConfig, !noAttach, kind)
			if sess != nil && !noAttach {
				fmt.Printf("session %s still running; cell attach --session %s / cell stop --session %s\n",
					sess.SessionID, sess.SessionID, sess.SessionID)
			}
			if err != nil {
				return err
			}
			if noAttach {
				fmt.Printf("Session %s ready at %s\n", sess.SessionID, sess.NetworkConfig.GuestIP)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&repo, "repo", "", "Source repository directory")
	agentKind = cfg.CellAgent
	cmd.Flags().StringVar(&agentKind, "agent", agentKind, "In-guest agent: opencode | claude | none")
	cmd.Flags().StringVar(&configPath, "config", "", "OpenCode JSON for guest ~/.config/opencode/opencode.json")
	cmd.Flags().BoolVar(&noAttach, "no-attach", false, "Wait for ready but do not exec SSH")
	_ = cmd.MarkFlagRequired("repo")
	return cmd
}
