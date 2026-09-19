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
}

func TestLoadArtifactPinDefaults(t *testing.T) {
	t.Setenv("CELL_CI_PREFIX", "")
	t.Setenv("CELL_KERNEL_VERSION", "")
	t.Setenv("CELL_FIRECRACKER_VERSION", "")

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
}

func TestLoadArtifactPinEnvOverrides(t *testing.T) {
	t.Setenv("CELL_CI_PREFIX", "firecracker-ci/20260624-ce269725504a-0/")
	t.Setenv("CELL_KERNEL_VERSION", "6.1.174")
	t.Setenv("CELL_FIRECRACKER_VERSION", "v1.16.0")

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
}

func TestLoadAgentServeDefaults(t *testing.T) {
	t.Setenv("CELL_AGENT_SERVE_PORT", "")
	t.Setenv("CELL_HOST_AGENT_BIN", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.AgentServePort != 4096 {
		t.Fatalf("AgentServePort = %d", cfg.AgentServePort)
	}
	if cfg.HostAgentBin != "opencode" {
		t.Fatalf("HostAgentBin = %q", cfg.HostAgentBin)
	}
	if cfg.ProjectDiskSizeMB != 3072 {
		t.Fatalf("ProjectDiskSizeMB = %d, want 3072", cfg.ProjectDiskSizeMB)
	}
	if cfg.RootfsSizeMB != 4096 {
		t.Fatalf("RootfsSizeMB = %d, want 4096", cfg.RootfsSizeMB)
	}
	if cfg.NodeVersion != "v24.20.0" {
		t.Fatalf("NodeVersion = %q, want v24.20.0", cfg.NodeVersion)
	}
	if cfg.UvVersion != "0.12.7" {
		t.Fatalf("UvVersion = %q, want 0.12.7", cfg.UvVersion)
	}
	if cfg.PythonVersion != "3.13" {
		t.Fatalf("PythonVersion = %q, want 3.13", cfg.PythonVersion)
	}
}

func TestLoadAgentServeEnvOverrides(t *testing.T) {
	t.Setenv("CELL_AGENT_SERVE_PORT", "4097")
	t.Setenv("CELL_HOST_AGENT_BIN", "/usr/local/bin/opencode")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.AgentServePort != 4097 {
		t.Fatalf("AgentServePort = %d", cfg.AgentServePort)
	}
	if cfg.HostAgentBin != "/usr/local/bin/opencode" {
		t.Fatalf("HostAgentBin = %q", cfg.HostAgentBin)
	}
}

func TestLoadDiskAndNodeEnvOverrides(t *testing.T) {
	t.Setenv("CELL_PROJECT_DISK_SIZE_MB", "2048")
	t.Setenv("CELL_ROOTFS_SIZE_MB", "5120")
	t.Setenv("CELL_NODE_VERSION", "v24.18.0")
	t.Setenv("CELL_UV_VERSION", "0.12.0")
	t.Setenv("CELL_PYTHON_VERSION", "3.12")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.ProjectDiskSizeMB != 2048 {
		t.Fatalf("ProjectDiskSizeMB = %d", cfg.ProjectDiskSizeMB)
	}
	if cfg.RootfsSizeMB != 5120 {
		t.Fatalf("RootfsSizeMB = %d", cfg.RootfsSizeMB)
	}
	if cfg.NodeVersion != "v24.18.0" {
		t.Fatalf("NodeVersion = %q", cfg.NodeVersion)
	}
	if cfg.UvVersion != "0.12.0" {
		t.Fatalf("UvVersion = %q", cfg.UvVersion)
	}
	if cfg.PythonVersion != "3.12" {
		t.Fatalf("PythonVersion = %q", cfg.PythonVersion)
	}
}

func TestInstallSuperpowersDefaultFalseAndEnvOverride(t *testing.T) {
	t.Setenv("CELL_INSTALL_SUPERPOWERS", "")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.InstallSuperpowers {
		t.Fatal("InstallSuperpowers must default to false")
	}
	t.Setenv("CELL_INSTALL_SUPERPOWERS", "true")
	cfg, err = Load()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.InstallSuperpowers {
		t.Fatal("CELL_INSTALL_SUPERPOWERS=true must set InstallSuperpowers")
	}
}
