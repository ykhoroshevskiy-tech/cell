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

func TestPrepareNetworkLockForGroup(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("requires root to chown lock file")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "cell-network.lock")
	restore := network.SetLockPathForTest(path)
	defer restore()

	if err := network.PrepareNetworkLockForGroup(os.Getgid()); err != nil {
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

func TestChownRootCellSetgid(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("requires root to chown directories")
	}
	dir := filepath.Join(t.TempDir(), "cell-data")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := network.ChownRootCell(dir, os.Getgid(), 02775); err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode()&os.ModeSetgid == 0 {
		t.Fatal("expected setgid bit")
	}
}
