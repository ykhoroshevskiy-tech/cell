package config

import "testing"

func TestLoadEnvOverridesArtifactPaths(t *testing.T) {
	t.Setenv("CELL_DATA_DIR", "/tmp/cell-data")
	t.Setenv("CELL_IMAGES_DIR", "/tmp/cell-images")
	t.Setenv("CELL_KERNEL_PATH", "/tmp/alt-vmlinux")
	t.Setenv("CELL_ROOTFS_PATH", "/tmp/alt-rootfs.ext4")
	t.Setenv("CELL_FIRECRACKER_BIN", "/tmp/alt-firecracker")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.DataDir != "/tmp/cell-data" {
		t.Fatalf("DataDir = %q, want %q", cfg.DataDir, "/tmp/cell-data")
	}
	if cfg.ImagesDir != "/tmp/cell-images" {
		t.Fatalf("ImagesDir = %q, want %q", cfg.ImagesDir, "/tmp/cell-images")
	}
	if cfg.KernelPath != "/tmp/alt-vmlinux" {
		t.Fatalf("KernelPath = %q, want %q", cfg.KernelPath, "/tmp/alt-vmlinux")
	}
	if cfg.RootfsPath != "/tmp/alt-rootfs.ext4" {
		t.Fatalf("RootfsPath = %q, want %q", cfg.RootfsPath, "/tmp/alt-rootfs.ext4")
	}
	if cfg.FirecrackerBin != "/tmp/alt-firecracker" {
		t.Fatalf("FirecrackerBin = %q, want %q", cfg.FirecrackerBin, "/tmp/alt-firecracker")
	}
}

func TestLoadDerivesArtifactPathsFromImagesDir(t *testing.T) {
	t.Setenv("CELL_DATA_DIR", "/tmp/cell-data")
	t.Setenv("CELL_IMAGES_DIR", "/tmp/cell-images")
	t.Setenv("CELL_KERNEL_PATH", "")
	t.Setenv("CELL_ROOTFS_PATH", "")
	t.Setenv("CELL_FIRECRACKER_BIN", "")
	t.Setenv("CELL_SQUASHFS_PATH", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.SessionDataDir != "/tmp/cell-data/session-data" {
		t.Fatalf("SessionDataDir = %q, want %q", cfg.SessionDataDir, "/tmp/cell-data/session-data")
	}
	if cfg.KernelPath != "/tmp/cell-images/vmlinux" {
		t.Fatalf("KernelPath = %q, want %q", cfg.KernelPath, "/tmp/cell-images/vmlinux")
	}
	if cfg.RootfsPath != "/tmp/cell-images/rootfs.ext4" {
		t.Fatalf("RootfsPath = %q, want %q", cfg.RootfsPath, "/tmp/cell-images/rootfs.ext4")
	}
	if cfg.FirecrackerBin != "/tmp/cell-images/bin/firecracker" {
		t.Fatalf("FirecrackerBin = %q, want %q", cfg.FirecrackerBin, "/tmp/cell-images/bin/firecracker")
	}
	if cfg.SquashfsPath != "/tmp/cell-images/ubuntu-22.04.squashfs" {
		t.Fatalf("SquashfsPath = %q, want %q", cfg.SquashfsPath, "/tmp/cell-images/ubuntu-22.04.squashfs")
	}
}
