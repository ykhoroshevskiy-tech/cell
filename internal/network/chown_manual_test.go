package network_test

import (
	"os"
	"strings"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/ykhoroshevskiy-tech/cell/internal/network"
)

func TestChownRootCellAsCappedRootManual(t *testing.T) {
	if os.Getenv("CELL_CHOWNTEST") != "1" {
		t.Skip("manual captest only")
	}
	if os.Geteuid() != 0 {
		t.Skip("requires root")
	}
	if out, err := os.ReadFile("/proc/self/status"); err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			if strings.HasPrefix(line, "CapEff:") {
				t.Logf("root process CapEff: %s", line)
			}
		}
	}
	path := filepath.Join(t.TempDir(), "f")
	if err := os.WriteFile(path, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := network.ChownRootCell(path, 990, 0664); err != nil {
		t.Fatalf("ChownRootCell: %v", err)
	}
	st, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if st.Sys().(*syscall.Stat_t).Gid != 990 {
		t.Fatalf("gid=%d want 990", st.Sys().(*syscall.Stat_t).Gid)
	}
	if st.Mode().Perm() != 0664 {
		t.Fatalf("perm=%o want 664", st.Mode().Perm())
	}
}
