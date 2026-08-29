package sync_test

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/ykhoroshevskiy-tech/cell/internal/sync"
)

func TestPullRsyncExcludesFilter(t *testing.T) {
	got := sync.PullRsyncExcludes()
	want := []string{"--exclude=.filter-staged", "--exclude=.filter"}
	if len(got) != len(want) {
		t.Fatalf("PullRsyncExcludes() = %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("PullRsyncExcludes() = %q, want %q", got, want)
		}
	}
}

func TestValidateDestUnsafe(t *testing.T) {
	for _, dest := range []string{"/", "/usr", "/bin", "/etc", "/var", "/sbin"} {
		if err := sync.ValidateDest(dest); err == nil {
			t.Fatalf("expected unsafe dest error for %q", dest)
		}
	}
	if err := sync.ValidateDest("/tmp/cell-pull"); err != nil {
		t.Fatalf("safe dest rejected: %v", err)
	}
}

func TestDestOwner(t *testing.T) {
	dir := t.TempDir()
	fi, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		t.Fatal("expected *syscall.Stat_t")
	}
	wantUID, wantGID := int(st.Uid), int(st.Gid)

	uid, gid, err := sync.DestOwner(dir)
	if err != nil {
		t.Fatal(err)
	}
	if uid != wantUID || gid != wantGID {
		t.Fatalf("DestOwner(%q) = %d:%d want %d:%d", dir, uid, gid, wantUID, wantGID)
	}

	missing := filepath.Join(dir, "not-yet-created")
	uid, gid, err = sync.DestOwner(missing)
	if err != nil {
		t.Fatal(err)
	}
	if uid != wantUID || gid != wantGID {
		t.Fatalf("DestOwner(%q) = %d:%d want parent %d:%d", missing, uid, gid, wantUID, wantGID)
	}
}
