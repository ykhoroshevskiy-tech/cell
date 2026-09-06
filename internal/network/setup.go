package network

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

var netSetupHook = netSetupImpl

// SetNetSetupForTest replaces NetSetup (tests only).
func SetNetSetupForTest(fn func() error) func() {
	old := netSetupHook
	netSetupHook = fn
	return func() { netSetupHook = old }
}

// NetSetup prepares the network lock file for runtime commands.
func NetSetup() error {
	return netSetupHook()
}

func netSetupImpl() error {
	return PrepareNetworkLock()
}

// NetSetupFull prepares the lock and converges bridge/firewall (hidden CLI).
func NetSetupFull() error {
	if err := PrepareNetworkLock(); err != nil {
		return err
	}
	if err := EnsureBridge(); err != nil {
		return fmt.Errorf("bridge: %w", err)
	}
	if err := EnsureFirewall(); err != nil {
		return fmt.Errorf("firewall: %w", err)
	}
	return nil
}

// PrepareNetworkLock creates or opens the lock without O_CREAT on an existing
// sticky /run/lock file (fs.protected_regular), then sets mode 0664. Runtime
// users skip the chmod on the bootstrap-owned root:cell lock: chmod requires
// ownership or CAP_FOWNER, and the lock's permissions are bootstrap's contract.
func PrepareNetworkLock() error {
	f, err := openLockFile()
	if err != nil {
		return err
	}
	defer f.Close()
	st, err := os.Stat(networkLockPath)
	if err != nil {
		return fmt.Errorf("stat network lock: %w", err)
	}
	if !shouldChmodLock(lockOwnerUid(st), uint32(os.Geteuid())) {
		return nil
	}
	if err := os.Chmod(networkLockPath, 0664); err != nil {
		return fmt.Errorf("chmod network lock: %w", err)
	}
	return nil
}

// shouldChmodLock reports whether the caller may chmod the lock: only the file
// owner or root can.
func shouldChmodLock(ownerUid, euid uint32) bool {
	return euid == 0 || ownerUid == euid
}

func lockOwnerUid(st os.FileInfo) uint32 {
	stt, ok := st.Sys().(*syscall.Stat_t)
	if !ok {
		return 0
	}
	return stt.Uid
}

// PrepareNetworkLockForGroup creates the lock file owned by root:cell.
func PrepareNetworkLockForGroup(gid int) error {
	if err := PrepareNetworkLock(); err != nil {
		return err
	}
	return ChownRootCell(networkLockPath, gid, 0664)
}

// ChownRootCell sets owner root:cell and mode on path. gid must be the cell group id.
// When running as root, chown runs as an external command: a file-caps-limited
// root process lacks CAP_CHOWN, while root's children start with the full
// bounding set. chmod stays in-process (owner rule suffices for root).
func ChownRootCell(path string, gid int, mode os.FileMode) error {
	if os.Geteuid() == 0 {
		if out, err := exec.Command("chown", fmt.Sprintf("0:%d", gid), path).CombinedOutput(); err != nil {
			return fmt.Errorf("chown %s: %w\n%s", path, err, out)
		}
	} else if err := os.Chown(path, 0, gid); err != nil {
		return fmt.Errorf("chown %s: %w", path, err)
	}
	if err := os.Chmod(path, mode); err != nil {
		return fmt.Errorf("chmod %s: %w", path, err)
	}
	return nil
}
