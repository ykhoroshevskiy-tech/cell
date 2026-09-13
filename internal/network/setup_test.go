package network_test

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/ykhoroshevskiy-tech/cell/internal/network"
)

func TestPrepareNetworkLock(t *testing.T) {
	old := syscall.Umask(0022)
	t.Cleanup(func() { syscall.Umask(old) })
	dir := t.TempDir()
	path := filepath.Join(dir, "cell-network.lock")
	restore := network.SetLockPathForTest(path)
	defer restore()

	if err := network.PrepareNetworkLock(); err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm() != 0664 {
		t.Fatalf("perm = %o, want 0664", st.Mode().Perm())
	}
}
