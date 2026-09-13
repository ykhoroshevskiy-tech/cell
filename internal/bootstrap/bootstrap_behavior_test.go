package bootstrap

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ykhoroshevskiy-tech/cell/internal/config"
)

func TestShouldSkipManagedArtifactDownloadForCustomPath(t *testing.T) {
	dir := t.TempDir()
	customPath := filepath.Join(dir, "custom-vmlinux")
	if err := os.WriteFile(customPath, []byte("kernel"), 0644); err != nil {
		t.Fatal(err)
	}

	if !shouldSkipManagedArtifactDownload(customPath, filepath.Join(dir, "images", "vmlinux"), false) {
		t.Fatal("existing custom path should skip managed download")
	}
}

func TestShouldNotSkipManagedArtifactDownloadForManagedDefault(t *testing.T) {
	dir := t.TempDir()
	managedPath := filepath.Join(dir, "images", "vmlinux")
	if err := os.MkdirAll(filepath.Dir(managedPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(managedPath, []byte("kernel"), 0644); err != nil {
		t.Fatal(err)
	}

	if shouldSkipManagedArtifactDownload(managedPath, managedPath, false) {
		t.Fatal("managed default path must not skip download just because the file exists")
	}
}

func TestNeedsRootfsRebuildWhenSquashfsStampMismatches(t *testing.T) {
	dir := t.TempDir()
	rootfsPath := filepath.Join(dir, "rootfs.ext4")
	if err := os.WriteFile(rootfsPath, []byte("rootfs"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(rootfsSquashfsStampPath(rootfsPath), []byte("24.04"), 0644); err != nil {
		t.Fatal(err)
	}

	needs, err := needsRootfsRebuild(rootfsPath, "debootstrap:noble+apt+node:v24.20.0", false)
	if err != nil {
		t.Fatalf("needsRootfsRebuild() error = %v", err)
	}
	if !needs {
		t.Fatal("stamp mismatch should force rootfs rebuild")
	}
}

func TestSquashfsBuildStampUsesNodePin(t *testing.T) {
	cfg := &config.CellConfig{
		NodeVersion: "v24.18.0",
	}

	if got := squashfsBuildStamp(cfg); got != "debootstrap:noble+apt+node:v24.18.0+uv:0.12.7+py:3.13+sp:off" {
		t.Fatalf("squashfsBuildStamp() = %q", got)
	}
}

func TestSquashfsBuildStampIncludesUvAndPython(t *testing.T) {
	cfg := &config.CellConfig{
		NodeVersion:   "v24.20.0",
		UvVersion:     "0.12.7",
		PythonVersion: "3.13",
	}
	want := "debootstrap:noble+apt+node:v24.20.0+uv:0.12.7+py:3.13+sp:off"
	if got := squashfsBuildStamp(cfg); got != want {
		t.Fatalf("squashfsBuildStamp() = %q, want %q", got, want)
	}
}
