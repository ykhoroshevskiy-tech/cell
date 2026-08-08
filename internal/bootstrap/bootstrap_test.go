package bootstrap_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ykhoroshevskiy-tech/cell/internal/bootstrap"
)

func TestValidateKernelELF(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "vmlinux")
	if err := os.WriteFile(path, []byte("\x7fELFfake"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := bootstrap.ValidateKernel(path); err != nil {
		t.Fatalf("expected valid ELF: %v", err)
	}
	if err := bootstrap.ValidateKernel(filepath.Join(dir, "missing")); err == nil {
		t.Fatal("expected missing kernel error")
	}
}

func TestSha256MatchesPlaceholder(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "artifact")
	if err := os.WriteFile(path, []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}
	if !bootstrap.Sha256MatchesForTest(path, "<hex>") {
		t.Fatal("placeholder sha256 should skip verify")
	}
}
