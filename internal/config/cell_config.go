package config

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type CellConfig struct {
	RuntimeRoot         string        `mapstructure:"runtime_root"`
	DataDir             string        `mapstructure:"data_dir"`
	ImagesDir           string        `mapstructure:"images_dir"`
	SessionDataDir      string        `mapstructure:"session_data_dir"`
	FirecrackerBin      string        `mapstructure:"firecracker_bin"`
	KernelPath          string        `mapstructure:"kernel_path"`
	RootfsPath          string        `mapstructure:"rootfs_path"`
	SquashfsPath        string        `mapstructure:"squashfs_path"`
	VCPUCount           int           `mapstructure:"vcpu_count"`
	MemSizeMiB          int           `mapstructure:"mem_size_mib"`
	ProjectDiskSizeMB   int           `mapstructure:"project_disk_size_mb"`
	BootTimeoutSec      time.Duration `mapstructure:"boot_timeout_sec"`
	SSHReadyTimeoutSec  time.Duration `mapstructure:"ssh_ready_timeout_sec"`
	GuestProjectMount   string        `mapstructure:"guest_project_mount"`
	GuestRepoDir        string        `mapstructure:"guest_repo_dir"`
	GuestAttachScript   string        `mapstructure:"guest_attach_script"`
	GuestProjectDevice  string        `mapstructure:"guest_project_device"`
	SSHUser             string        `mapstructure:"ssh_user"`
	TmuxSessionName     string        `mapstructure:"tmux_session_name"`
	IncludeGit          bool          `mapstructure:"include_git"`
	ExcludePatterns     []string      `mapstructure:"exclude_patterns"`
	AutoPull            bool          `mapstructure:"auto_pull"`
	AutoPullIntervalSec int           `mapstructure:"auto_pull_interval_sec"`
	RebuildRootfs       bool          `mapstructure:"rebuild_rootfs"`
	SSHPublicKey        string        `mapstructure:"ssh_public_key"`
	CIPrefix            string        `mapstructure:"ci_prefix"`
	KernelVersion       string        `mapstructure:"kernel_version"`
	FirecrackerVersion  string        `mapstructure:"firecracker_version"`
	SquashfsVersion     string        `mapstructure:"squashfs_version"`
	// Agent install/attach (vendor-neutral; defaults target OpenCode).
	AgentURL string `mapstructure:"agent_url"` // empty = skip install; may contain {target}
	AgentBin string `mapstructure:"agent_bin"` // binary name inside tarball and on PATH
	AgentCmd string `mapstructure:"agent_cmd"` // command run in tmux on attach
	AgentServePort int    `mapstructure:"agent_serve_port"`
	HostAgentBin   string `mapstructure:"host_agent_bin"`
}

func managedSquashfsPath(imagesDir, version string) string {
	return filepath.Join(imagesDir, "ubuntu-"+version+".squashfs")
}

func Default() *CellConfig {
	runtimeRoot := detectRuntimeRoot()
	dataDir := "/var/lib/cell"
	imagesDir := filepath.Join(dataDir, "images")
	return &CellConfig{
		RuntimeRoot:         runtimeRoot,
		DataDir:             dataDir,
		ImagesDir:           imagesDir,
		SessionDataDir:      filepath.Join(dataDir, "session-data"),
		FirecrackerBin:      filepath.Join(imagesDir, "bin", "firecracker"),
		KernelPath:          filepath.Join(imagesDir, "vmlinux"),
		RootfsPath:          filepath.Join(imagesDir, "rootfs.ext4"),
		SquashfsPath:        managedSquashfsPath(imagesDir, "24.04"),
		VCPUCount:           4,
		MemSizeMiB:          8192,
		ProjectDiskSizeMB:   1024,
		BootTimeoutSec:      120 * time.Second,
		SSHReadyTimeoutSec:  90 * time.Second,
		GuestProjectMount:   "/project",
		GuestRepoDir:        "/project",
		GuestAttachScript:   "/opt/guest-init/tmux-attach.sh",
		GuestProjectDevice:  "/dev/vdb",
		SSHUser:             "agent",
		TmuxSessionName:     "agent",
		IncludeGit:          true,
		ExcludePatterns:     defaultExcludePatterns(),
		AutoPull:            true,
		AutoPullIntervalSec: 30,
		RebuildRootfs:       false,
		CIPrefix:            "firecracker-ci/20260708-f11c230ed107-0/",
		KernelVersion:       "6.1.176",
		FirecrackerVersion:  "v1.16.1",
		SquashfsVersion:     "24.04",
		AgentURL:            "https://github.com/anomalyco/opencode/releases/latest/download/opencode-{target}.tar.gz",
		AgentBin:            "opencode",
		AgentCmd:            "opencode serve --hostname 127.0.0.1 --port 4096",
		AgentServePort:      4096,
		HostAgentBin:        "opencode",
	}
}

func defaultExcludePatterns() []string {
	return []string{
		"__pycache__", ".venv", "node_modules", ".pytest_cache", "session-data", "images",
	}
}

func detectRuntimeRoot() string {
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	// runtime/cell/cell -> repo root
	return filepath.Clean(filepath.Join(filepath.Dir(exe), "..", "..", ".."))
}

func Load() (*CellConfig, error) {
	v := viper.New()
	v.SetEnvPrefix("CELL")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	keys := []string{
		"runtime_root", "data_dir", "images_dir", "session_data_dir",
		"firecracker_bin", "kernel_path", "rootfs_path", "squashfs_path",
		"vcpu_count", "mem_size_mib", "project_disk_size_mb",
		"boot_timeout_sec", "ssh_ready_timeout_sec",
		"guest_project_mount", "guest_repo_dir", "guest_attach_script", "guest_project_device",
		"ssh_user", "tmux_session_name", "include_git", "exclude_patterns",
		"auto_pull", "auto_pull_interval_sec", "rebuild_rootfs", "ssh_public_key",
		"ci_prefix", "kernel_version", "firecracker_version", "squashfs_version",
		"agent_url", "agent_bin", "agent_cmd",
		"agent_serve_port", "host_agent_bin",
	}
	for _, k := range keys {
		_ = v.BindEnv(k)
	}

	def := Default()
	v.SetDefault("runtime_root", def.RuntimeRoot)
	v.SetDefault("data_dir", def.DataDir)
	v.SetDefault("vcpu_count", def.VCPUCount)
	v.SetDefault("mem_size_mib", def.MemSizeMiB)
	v.SetDefault("project_disk_size_mb", def.ProjectDiskSizeMB)
	v.SetDefault("boot_timeout_sec", def.BootTimeoutSec)
	v.SetDefault("ssh_ready_timeout_sec", def.SSHReadyTimeoutSec)
	v.SetDefault("guest_project_mount", def.GuestProjectMount)
	v.SetDefault("guest_repo_dir", def.GuestRepoDir)
	v.SetDefault("guest_attach_script", def.GuestAttachScript)
	v.SetDefault("guest_project_device", def.GuestProjectDevice)
	v.SetDefault("ssh_user", def.SSHUser)
	v.SetDefault("tmux_session_name", def.TmuxSessionName)
	v.SetDefault("include_git", def.IncludeGit)
	v.SetDefault("exclude_patterns", def.ExcludePatterns)
	v.SetDefault("auto_pull", def.AutoPull)
	v.SetDefault("auto_pull_interval_sec", def.AutoPullIntervalSec)
	v.SetDefault("rebuild_rootfs", def.RebuildRootfs)
	v.SetDefault("ssh_public_key", def.SSHPublicKey)
	v.SetDefault("ci_prefix", def.CIPrefix)
	v.SetDefault("kernel_version", def.KernelVersion)
	v.SetDefault("firecracker_version", def.FirecrackerVersion)
	v.SetDefault("squashfs_version", def.SquashfsVersion)
	v.SetDefault("agent_url", def.AgentURL)
	v.SetDefault("agent_bin", def.AgentBin)
	v.SetDefault("agent_cmd", def.AgentCmd)
	v.SetDefault("agent_serve_port", def.AgentServePort)
	v.SetDefault("host_agent_bin", def.HostAgentBin)

	cfg := &CellConfig{}
	if err := v.Unmarshal(cfg); err != nil {
		return nil, err
	}
	if cfg.RuntimeRoot == "" {
		cfg.RuntimeRoot = def.RuntimeRoot
	}
	if cfg.DataDir == "" {
		cfg.DataDir = def.DataDir
	}
	if cfg.ImagesDir == "" {
		cfg.ImagesDir = filepath.Join(cfg.DataDir, "images")
	}
	if cfg.SessionDataDir == "" {
		cfg.SessionDataDir = filepath.Join(cfg.DataDir, "session-data")
	}
	if cfg.FirecrackerBin == "" {
		cfg.FirecrackerBin = filepath.Join(cfg.ImagesDir, "bin", "firecracker")
	}
	if cfg.KernelPath == "" {
		cfg.KernelPath = filepath.Join(cfg.ImagesDir, "vmlinux")
	}
	if cfg.RootfsPath == "" {
		cfg.RootfsPath = filepath.Join(cfg.ImagesDir, "rootfs.ext4")
	}
	if cfg.CIPrefix == "" {
		cfg.CIPrefix = def.CIPrefix
	}
	if cfg.KernelVersion == "" {
		cfg.KernelVersion = def.KernelVersion
	}
	if cfg.FirecrackerVersion == "" {
		cfg.FirecrackerVersion = def.FirecrackerVersion
	}
	if cfg.SquashfsVersion == "" {
		cfg.SquashfsVersion = def.SquashfsVersion
	}
	if cfg.SquashfsPath == "" {
		cfg.SquashfsPath = managedSquashfsPath(cfg.ImagesDir, cfg.SquashfsVersion)
	}
	if cfg.GuestAttachScript == "" {
		cfg.GuestAttachScript = def.GuestAttachScript
	}
	if cfg.TmuxSessionName == "" {
		cfg.TmuxSessionName = def.TmuxSessionName
	}
	if cfg.AgentBin == "" {
		cfg.AgentBin = def.AgentBin
	}
	if cfg.AgentCmd == "" {
		cfg.AgentCmd = def.AgentCmd
	}
	if cfg.AgentServePort == 0 {
		cfg.AgentServePort = 4096
	}
	if cfg.HostAgentBin == "" {
		cfg.HostAgentBin = "opencode"
	}
	// AgentURL: empty after env means skip install; only fill default when unset via SetDefault.
	// viper leaves "" if CELL_AGENT_URL="" intentionally — do not replace empty with default here.
	return cfg, nil
}
