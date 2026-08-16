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
	if cfg.SquashfsPath != "/tmp/cell-images/ubuntu-24.04.squashfs" {
		t.Fatalf("SquashfsPath = %q, want %q", cfg.SquashfsPath, "/tmp/cell-images/ubuntu-24.04.squashfs")
	}
}

func TestLoadArtifactPinDefaults(t *testing.T) {
	t.Setenv("CELL_CI_PREFIX", "")
	t.Setenv("CELL_KERNEL_VERSION", "")
	t.Setenv("CELL_FIRECRACKER_VERSION", "")
	t.Setenv("CELL_SQUASHFS_VERSION", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.CIPrefix != "firecracker-ci/20260708-f11c230ed107-0/" {
		t.Fatalf("CIPrefix = %q", cfg.CIPrefix)
	}
	if cfg.KernelVersion != "6.1.176" {
		t.Fatalf("KernelVersion = %q", cfg.KernelVersion)
	}
	if cfg.FirecrackerVersion != "v1.16.1" {
		t.Fatalf("FirecrackerVersion = %q", cfg.FirecrackerVersion)
	}
	if cfg.SquashfsVersion != "24.04" {
		t.Fatalf("SquashfsVersion = %q", cfg.SquashfsVersion)
	}
}

func TestLoadArtifactPinEnvOverrides(t *testing.T) {
	t.Setenv("CELL_CI_PREFIX", "firecracker-ci/20260624-ce269725504a-0/")
	t.Setenv("CELL_KERNEL_VERSION", "6.1.174")
	t.Setenv("CELL_FIRECRACKER_VERSION", "v1.16.0")
	t.Setenv("CELL_SQUASHFS_VERSION", "22.04")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.CIPrefix != "firecracker-ci/20260624-ce269725504a-0/" {
		t.Fatalf("CIPrefix = %q", cfg.CIPrefix)
	}
	if cfg.KernelVersion != "6.1.174" {
		t.Fatalf("KernelVersion = %q", cfg.KernelVersion)
	}
	if cfg.FirecrackerVersion != "v1.16.0" {
		t.Fatalf("FirecrackerVersion = %q", cfg.FirecrackerVersion)
	}
	if cfg.SquashfsVersion != "22.04" {
		t.Fatalf("SquashfsVersion = %q", cfg.SquashfsVersion)
	}
}

func TestLoadAgentServeDefaults(t *testing.T) {
	t.Setenv("CELL_AGENT_CMD", "")
	t.Setenv("CELL_AGENT_SERVE_PORT", "")
	t.Setenv("CELL_HOST_AGENT_BIN", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.AgentCmd != "opencode serve --hostname 127.0.0.1 --port 4096" {
		t.Fatalf("AgentCmd = %q", cfg.AgentCmd)
	}
	if cfg.AgentServePort != 4096 {
		t.Fatalf("AgentServePort = %d", cfg.AgentServePort)
	}
	if cfg.HostAgentBin != "opencode" {
		t.Fatalf("HostAgentBin = %q", cfg.HostAgentBin)
	}
}

func TestLoadAgentServeEnvOverrides(t *testing.T) {
	t.Setenv("CELL_AGENT_CMD", "opencode serve --hostname 127.0.0.1 --port 4097")
	t.Setenv("CELL_AGENT_SERVE_PORT", "4097")
	t.Setenv("CELL_HOST_AGENT_BIN", "/usr/local/bin/opencode")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.AgentCmd != "opencode serve --hostname 127.0.0.1 --port 4097" {
		t.Fatalf("AgentCmd = %q", cfg.AgentCmd)
	}
	if cfg.AgentServePort != 4097 {
		t.Fatalf("AgentServePort = %d", cfg.AgentServePort)
	}
	if cfg.HostAgentBin != "/usr/local/bin/opencode" {
		t.Fatalf("HostAgentBin = %q", cfg.HostAgentBin)
	}
}

func TestLoadDerivesManagedSquashfsPathFromVersion(t *testing.T) {
	t.Setenv("CELL_IMAGES_DIR", "/tmp/cell-images")
	t.Setenv("CELL_SQUASHFS_VERSION", "22.04")
	t.Setenv("CELL_SQUASHFS_PATH", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.SquashfsPath != "/tmp/cell-images/ubuntu-22.04.squashfs" {
		t.Fatalf("SquashfsPath = %q, want %q", cfg.SquashfsPath, "/tmp/cell-images/ubuntu-22.04.squashfs")
	}
}
