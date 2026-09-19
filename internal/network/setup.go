package network

import (
	"fmt"
	"os"
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
// sticky /run/lock file (fs.protected_regular), then sets mode 0664.
func PrepareNetworkLock() error {
	f, err := openLockFile()
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	if err := os.Chmod(networkLockPath, 0664); err != nil {
		return fmt.Errorf("chmod network lock: %w", err)
	}
	return nil
}
