package bootstrap

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"

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

	if err := migrateLegacySessionOwnership(cfg.SessionDataDir); err != nil {
		return err
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

	groups := runtimeGroups()
	if sudoUser := os.Getenv("SUDO_USER"); sudoUser != "" {
		joined := strings.Join(groups, ",")
		cmd := exec.Command("usermod", "-aG", joined, sudoUser)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("usermod -aG %s %s: %w\n%s", joined, sudoUser, err, out)
		}
		for _, g := range groups {
			fmt.Printf("added %s to group %s (re-login or newgrp %s)\n", sudoUser, g, g)
		}
	} else {
		fmt.Printf("add your user to groups %s: sudo usermod -aG %s \"$USER\" && newgrp %s\n",
			strings.Join(groups, ","), strings.Join(groups, ","), privilege.CellGroupName)
	}

	fmt.Println("✓ rootless runtime environment ready")
	return nil
}

// runtimeGroups returns the host groups a runtime user needs: cell plus kvm
// when /dev/kvm exists (firecracker runs as the invoking user, not root).
func runtimeGroups() []string {
	groups := []string{privilege.CellGroupName}
	if _, err := os.Stat("/dev/kvm"); err != nil {
		return groups
	}
	if _, err := user.LookupGroup("kvm"); err != nil {
		return groups
	}
	return append(groups, "kvm")
}

// migrateLegacySessionOwnership hands files from earlier sudo-created sessions
// to the cell group so runtime commands can manage them without sudo. SSH
// private keys are re-tightened to 0600 afterwards because ssh rejects
// group-readable keys.
func migrateLegacySessionOwnership(sessionDataDir string) error {
	st, err := os.Stat(sessionDataDir)
	if err != nil || !st.IsDir() {
		return nil
	}
	if out, err := exec.Command("chgrp", "-R", privilege.CellGroupName, sessionDataDir).CombinedOutput(); err != nil {
		return fmt.Errorf("chgrp -R %s %s: %w\n%s", privilege.CellGroupName, sessionDataDir, err, out)
	}
	if out, err := exec.Command("chmod", "-R", "g+rwX", sessionDataDir).CombinedOutput(); err != nil {
		return fmt.Errorf("chmod -R g+rwX %s: %w\n%s", sessionDataDir, err, out)
	}
	return retightenSSHKeys(sessionDataDir)
}

func retightenSSHKeys(sessionDataDir string) error {
	return filepath.WalkDir(sessionDataDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || d.Name() != "id_ed25519" {
			return nil
		}
		return os.Chmod(path, 0600)
	})
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
		if err := network.ChownRootCell(p, gid, 0750); err != nil {
			return err
		}
		fmt.Printf("capabilities on %s (mode 0750 root:%s)\n", p, privilege.CellGroupName)
	}
	return nil
}
