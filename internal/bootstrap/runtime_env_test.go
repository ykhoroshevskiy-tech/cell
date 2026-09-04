package bootstrap_test

import (
	"os"
	"strings"
	"testing"

	"github.com/ykhoroshevskiy-tech/cell/internal/bootstrap"
	"github.com/ykhoroshevskiy-tech/cell/internal/config"
)

func TestPrepareRuntimeEnvironmentRequiresRoot(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("requires non-root")
	}
	cfg := config.Default()
	err := bootstrap.PrepareRuntimeEnvironment(cfg)
	if err == nil {
		t.Fatal("expected error when not root")
	}
	if !strings.Contains(err.Error(), "requires root") {
		t.Fatalf("err=%v", err)
	}
}
