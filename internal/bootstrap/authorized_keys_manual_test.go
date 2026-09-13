package bootstrap

import (
	"os"
	"testing"

	"github.com/ykhoroshevskiy-tech/cell/internal/config"
)

func TestEnsureAuthorizedKeysRootOwnedManual(t *testing.T) {
	if os.Getenv("CELL_KEYTEST") != "1" {
		t.Skip("manual keytest only")
	}
	dir := "/tmp/cell-keytest/init-scripts"
	if err := ensureAuthorizedKeys(&config.CellConfig{}, dir); err != nil {
		t.Fatalf("non-root launch against root-owned init-scripts: %v", err)
	}
}
