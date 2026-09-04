package bootstrap

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/ykhoroshevskiy-tech/cell/internal/config"
	"github.com/ykhoroshevskiy-tech/cell/internal/network"
	"github.com/ykhoroshevskiy-tech/cell/internal/privilege"
)

const (
	cellSetgidDirPerm = 02775
	installedCellBin  = "/usr/bin/cell"
	// net caps inherit into ip/iptables/firecracker via ambient raise; dac_override stays on cell only.
	cellFileCaps = "cap_net_admin,cap_net_raw+eip cap_dac_override+ep"
)

// PrepareRuntimeEnvironment configures group-owned state dirs, network lock, capabilities, and group membership.
// Must run as root. Does not recurse into session files (SSH keys stay 0600 for their owner).
func PrepareRuntimeEnvironment(cfg *config.CellConfig) error {
	if os.Geteuid() != 0 {
		return fmt.Errorf("prepare runtime environment requires root")
	}
	if err := exec.Command("groupadd", "-f", privilege.CellGroupName).Run(); err != nil {
		return fmt.Errorf("groupadd %s: %w", privilege.CellGroupName, err)
	}
	gid, err := privilege.CellGroupGID()
	if err != nil {
		return err
	}

	dirs := []string{
		cfg.DataDir,
		cfg.SessionDataDir,
		cfg.ImagesDir,
		filepath.Join(cfg.ImagesDir, "bin"),
		filepath.Join(cfg.DataDir, "init-scripts"),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, cellSetgidDirPerm); err != nil {
			return fmt.Errorf("mkdir %s: %w", dir, err)
		}
		if err := network.ChownRootCell(dir, gid, cellSetgidDirPerm); err != nil {
			return err
		}
	}

	if err := network.PrepareNetworkLockForGroup(gid); err != nil {
		return err
	}
	if err := network.EnsureBridge(); err != nil {
		return err
	}
	if err := network.EnsureFirewall(); err != nil {
		return err
	}

	if err := applyFileCaps(gid); err != nil {
		return err
	}

	if sudoUser := os.Getenv("SUDO_USER"); sudoUser != "" {
		cmd := exec.Command("usermod", "-aG", privilege.CellGroupName, sudoUser)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("usermod -aG %s %s: %w\n%s", privilege.CellGroupName, sudoUser, err, out)
		}
		fmt.Printf("added %s to group %s (re-login or newgrp %s)\n", sudoUser, privilege.CellGroupName, privilege.CellGroupName)
	} else {
		fmt.Printf("add your user to group %s: sudo usermod -aG %s \"$USER\" && newgrp %s\n",
			privilege.CellGroupName, privilege.CellGroupName, privilege.CellGroupName)
	}

	fmt.Println("✓ rootless runtime environment ready")
	return nil
}

func applyFileCaps(gid int) error {
	seen := map[string]struct{}{}
	var paths []string
	add := func(p string) {
		if p == "" {
			return
		}
		st, err := os.Stat(p)
		if err != nil || st.IsDir() {
			return
		}
		if _, ok := seen[p]; ok {
			return
		}
		seen[p] = struct{}{}
		paths = append(paths, p)
	}
	if exe, err := os.Executable(); err == nil {
		add(exe)
	}
	add(installedCellBin)
	if len(paths) == 0 {
		return fmt.Errorf("no cell binary found to setcap (install to %s)", installedCellBin)
	}
	for _, p := range paths {
		setcap := exec.Command("setcap", cellFileCaps, p)
		if out, err := setcap.CombinedOutput(); err != nil {
			return fmt.Errorf("setcap %s: %w\n%s", p, err, out)
		}
		if err := os.Chown(p, 0, gid); err != nil {
			return fmt.Errorf("chown %s: %w", p, err)
		}
		if err := os.Chmod(p, 0750); err != nil {
			return fmt.Errorf("chmod 0750 %s: %w", p, err)
		}
		fmt.Printf("capabilities on %s (mode 0750 root:%s)\n", p, privilege.CellGroupName)
	}
	return nil
}
