package network

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

var networkLockPath = "/run/lock/cell-network.lock"

// SetLockPathForTest overrides the lock file path (tests only).
func SetLockPathForTest(path string) func() {
	old := networkLockPath
	networkLockPath = path
	return func() { networkLockPath = old }
}

func openLockFile() (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(networkLockPath), 0755); err != nil {
		return nil, fmt.Errorf("ensure lock dir: %w", err)
	}
	f, err := os.OpenFile(networkLockPath, os.O_RDWR, 0644)
	if err == nil {
		return f, nil
	}
	if !os.IsNotExist(err) {
		return nil, fmt.Errorf("open network lock: %w", err)
	}
	f, err = os.OpenFile(networkLockPath, os.O_CREATE|os.O_RDWR|os.O_EXCL, 0664)
	if err == nil {
		return f, nil
	}
	if os.IsExist(err) {
		f, err = os.OpenFile(networkLockPath, os.O_RDWR, 0644)
		if err != nil {
			return nil, fmt.Errorf("open network lock: %w", err)
		}
		return f, nil
	}
	return nil, fmt.Errorf("open network lock: %w", err)
}

// WithNetworkLock runs fn while holding the global cell network lock.
func WithNetworkLock(fn func() error) error {
	f, err := openLockFile()
	if err != nil {
		return err
	}
	defer f.Close()

	deadline := time.Now().Add(5 * time.Minute)
	for {
		err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("network lock timeout")
		}
		time.Sleep(100 * time.Millisecond)
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN) //nolint:errcheck
	return fn()
}
