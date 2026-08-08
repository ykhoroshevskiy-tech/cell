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
	cmd := &cobra.Command{
		Use:   "launch",
		Short: "Stage repo, boot VM, attach SSH, auto-pull",
		RunE: func(cmd *cobra.Command, args []string) error {
			if repo == "" {
				return fmt.Errorf("--repo is required")
			}
			verbose.V("cell launch repo=%s attach=%v", repo, !noAttach)
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

			sess, err := sm.Launch(ctx, repo, !noAttach)
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
	cmd.Flags().BoolVar(&noAttach, "no-attach", false, "Wait for ready but do not exec SSH")
	_ = cmd.MarkFlagRequired("repo")
	return cmd
}
