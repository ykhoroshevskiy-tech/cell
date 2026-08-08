package sync_test

import (
	"testing"

	"github.com/ykhoroshevskiy-tech/cell/internal/sync"
)

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
