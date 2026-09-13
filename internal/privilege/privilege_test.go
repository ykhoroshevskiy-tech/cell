package privilege_test

import (
	"os"
	"strings"
	"testing"

	"github.com/ykhoroshevskiy-tech/cell/internal/privilege"
)

func TestNeedsRuntimeAccess(t *testing.T) {
	if privilege.NeedsRuntimeAccess([]string{"cell"}) {
		t.Fatal("bare cell should not need runtime access")
	}
	if !privilege.NeedsRuntimeAccess([]string{"cell", "launch"}) {
		t.Fatal("launch should need runtime access")
	}
	if privilege.NeedsRuntimeAccess([]string{"cell", "bootstrap"}) {
		t.Fatal("bootstrap should be exempt")
	}
	if privilege.NeedsRuntimeAccess([]string{"cell", "rescue"}) {
		t.Fatal("rescue should be exempt")
	}
	if privilege.NeedsRuntimeAccess([]string{"cell", "version"}) {
		t.Fatal("version should be exempt")
	}
}

func TestRequireRuntimeAccess(t *testing.T) {
	if privilege.InCellGroup() {
		t.Skip("already in cell group")
	}
	old := os.Args
	os.Args = []string{"cell", "ps"}
	t.Cleanup(func() { os.Args = old })
	err := privilege.RequireRuntimeAccess()
	if err == nil || !strings.Contains(err.Error(), "runtime requires") {
		t.Fatalf("err=%v", err)
	}
	if strings.Contains(err.Error(), "sudo") {
		t.Fatal("runtime gate must not mention sudo")
	}
}

func TestRequireBootstrapRoot(t *testing.T) {
	if os.Geteuid() == 0 {
		if err := privilege.RequireBootstrapRoot(); err != nil {
			t.Fatalf("root should pass: %v", err)
		}
		return
	}
	err := privilege.RequireBootstrapRoot()
	if err == nil || !strings.Contains(err.Error(), "bootstrap requires root") {
		t.Fatalf("err=%v", err)
	}
}
