package network

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWithNetworkLockOpensExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cell-network.lock")
	if err := os.WriteFile(path, nil, 0644); err != nil {
		t.Fatal(err)
	}
	restore := SetLockPathForTest(path)
	defer restore()
	if err := WithNetworkLock(func() error { return nil }); err != nil {
		t.Fatal(err)
	}
}

func TestWithNetworkLockSerializes(t *testing.T) {
	restore := SetLockPathForTest(filepath.Join(t.TempDir(), "cell-network.lock"))
	defer restore()

	order := 0
	for i := 0; i < 3; i++ {
		n := i
		if err := WithNetworkLock(func() error {
			if order != n {
				t.Fatalf("order=%d want %d", order, n)
			}
			order++
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
}
