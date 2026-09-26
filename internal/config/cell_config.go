package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type CellConfig struct {
	DataDir             string        `mapstructure:"data_dir"`
	ImagesDir           string        `mapstructure:"images_dir"`
	SessionDataDir      string        `mapstructure:"session_data_dir"`
	FirecrackerBin      string        `mapstructure:"firecracker_bin"`
	KernelPath          string        `mapstructure:"kernel_path"`
	RootfsPath          string        `mapstructure:"rootfs_path"`
	VCPUCount           int           `mapstructure:"vcpu_count"`
	MemSizeMiB          int           `mapstructure:"mem_size_mib"`
	ProjectDiskSizeMB   int           `mapstructure:"project_disk_size_mb"`
	RootfsSizeMB        int           `mapstructure:"rootfs_size_mb"`
	SSHReadyTimeout     time.Duration `mapstructure:"ssh_ready_timeout_sec"`
	GuestRepoDir        string        `mapstructure:"guest_repo_dir"`
	GuestAttachScript   string        `mapstructure:"guest_attach_script"`
	SSHUser             string        `mapstructure:"ssh_user"`
	TmuxSessionName     string        `mapstructure:"tmux_session_name"`
	IncludeGit          bool          `mapstructure:"include_git"`
	ExcludePatterns     []string      `mapstructure:"exclude_patterns"`
	InstallSuperpowers  bool          `mapstructure:"install_superpowers"`
	AutoPull            bool          `mapstructure:"auto_pull"`
	AutoPullIntervalSec int           `mapstructure:"auto_pull_interval_sec"`
	RebuildRootfs       bool          `mapstructure:"rebuild_rootfs"`
	SSHPublicKey        string        `mapstructure:"ssh_public_key"`
	CIPrefix            string        `mapstructure:"ci_prefix"`
	KernelVersion       string        `mapstructure:"kernel_version"`
	FirecrackerVersion  string        `mapstructure:"firecracker_version"`
	NodeVersion         string        `mapstructure:"node_version"`
	UvVersion           string        `mapstructure:"uv_version"`
	PythonVersion       string        `mapstructure:"python_version"`
	// Agent install/attach for the opencode guest agent (claude needs none of these).
	AgentURL       string `mapstructure:"agent_url"` // opencode tarball URL; empty = skip install; may contain {target}
	AgentBin       string `mapstructure:"agent_bin"` // opencode binary name inside tarball and on PATH
	AgentServePort int    `mapstructure:"agent_serve_port"`
	HostAgentBin   string `mapstructure:"host_agent_bin"`
}

func Default() *CellConfig {
	dataDir := "/var/lib/cell"
	imagesDir := filepath.Join(dataDir, "images")
	return &CellConfig{
		DataDir:             dataDir,
		ImagesDir:           imagesDir,
		SessionDataDir:      filepath.Join(dataDir, "session-data"),
		FirecrackerBin:      filepath.Join(imagesDir, "bin", "firecracker"),
		KernelPath:          filepath.Join(imagesDir, "vmlinux"),
		RootfsPath:          filepath.Join(imagesDir, "rootfs.ext4"),
		VCPUCount:           4,
		MemSizeMiB:          8192,
		ProjectDiskSizeMB:   3072,
		RootfsSizeMB:        4096,
		SSHReadyTimeout:     90 * time.Second,
		GuestRepoDir:        "/project",
		GuestAttachScript:   "/opt/guest-init/tmux-attach.sh",
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
		NodeVersion:         "v24.20.0",
		UvVersion:           "0.12.7",
		PythonVersion:       "3.13",
		AgentURL:            defaultOpencodeAgentURL(),
		AgentBin:            "opencode",
		AgentServePort:      4096,
		HostAgentBin:        "opencode",
	}
}

const (
	// claudeCodeVersion pins the @anthropic-ai/claude-code npm tarball version
	// installed into the rootfs; CELL_AGENT_URL does not affect it.
	claudeCodeVersion = "1.0.98"
)

// DefaultClaudeAgentURL returns the pinned npm tarball URL for the claude
// agent installed alongside opencode during bootstrap.
func DefaultClaudeAgentURL() string {
	return fmt.Sprintf(
		"https://registry.npmjs.org/@anthropic-ai/claude-code/-/claude-code-%s.tgz",
		claudeCodeVersion)
}

// defaultOpencodeAgentURL returns the release tarball URL template for the
// opencode agent; it always tracks the latest release, {target} is replaced
// with the host arch at install time. CELL_AGENT_URL overrides the whole URL.
func defaultOpencodeAgentURL() string {
	return "https://github.com/anomalyco/opencode/releases/latest/download/opencode-{target}.tar.gz"
}

func defaultExcludePatterns() []string {
	return []string{
		"__pycache__", ".venv", "node_modules", ".pytest_cache", "session-data", "images",
	}
}

func Load() (*CellConfig, error) {
	v := viper.New()
	v.SetEnvPrefix("CELL")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	keys := []string{
		"data_dir", "images_dir", "session_data_dir",
		"firecracker_bin", "kernel_path", "rootfs_path",
		"vcpu_count", "mem_size_mib", "project_disk_size_mb", "rootfs_size_mb",
		"ssh_ready_timeout_sec",
		"guest_repo_dir", "guest_attach_script",
		"ssh_user", "tmux_session_name", "include_git", "exclude_patterns",
		"install_superpowers",
		"auto_pull", "auto_pull_interval_sec", "rebuild_rootfs", "ssh_public_key",
		"ci_prefix", "kernel_version", "firecracker_version", "node_version",
		"uv_version", "python_version",
		"agent_url", "agent_bin",
		"agent_serve_port", "host_agent_bin",
	}
	for _, k := range keys {
		_ = v.BindEnv(k)
	}

	def := Default()
	v.SetDefault("data_dir", def.DataDir)
	v.SetDefault("vcpu_count", def.VCPUCount)
	v.SetDefault("mem_size_mib", def.MemSizeMiB)
	v.SetDefault("project_disk_size_mb", def.ProjectDiskSizeMB)
	v.SetDefault("rootfs_size_mb", def.RootfsSizeMB)
	v.SetDefault("ssh_ready_timeout_sec", def.SSHReadyTimeout)
	v.SetDefault("guest_repo_dir", def.GuestRepoDir)
	v.SetDefault("guest_attach_script", def.GuestAttachScript)
	v.SetDefault("ssh_user", def.SSHUser)
	v.SetDefault("tmux_session_name", def.TmuxSessionName)
	v.SetDefault("include_git", def.IncludeGit)
	v.SetDefault("exclude_patterns", def.ExcludePatterns)
	v.SetDefault("install_superpowers", def.InstallSuperpowers)
	v.SetDefault("auto_pull", def.AutoPull)
	v.SetDefault("auto_pull_interval_sec", def.AutoPullIntervalSec)
	v.SetDefault("rebuild_rootfs", def.RebuildRootfs)
	v.SetDefault("ssh_public_key", def.SSHPublicKey)
	v.SetDefault("ci_prefix", def.CIPrefix)
	v.SetDefault("kernel_version", def.KernelVersion)
	v.SetDefault("firecracker_version", def.FirecrackerVersion)
	v.SetDefault("node_version", def.NodeVersion)
	v.SetDefault("uv_version", def.UvVersion)
	v.SetDefault("python_version", def.PythonVersion)
	v.SetDefault("agent_url", def.AgentURL)
	v.SetDefault("agent_bin", def.AgentBin)
	v.SetDefault("agent_serve_port", def.AgentServePort)
	v.SetDefault("host_agent_bin", def.HostAgentBin)

	cfg := &CellConfig{}
	if err := v.Unmarshal(cfg); err != nil {
		return nil, err
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
	if cfg.NodeVersion == "" {
		cfg.NodeVersion = def.NodeVersion
	}
	if cfg.UvVersion == "" {
		cfg.UvVersion = def.UvVersion
	}
	if cfg.PythonVersion == "" {
		cfg.PythonVersion = def.PythonVersion
	}
	if cfg.RootfsSizeMB == 0 {
		cfg.RootfsSizeMB = def.RootfsSizeMB
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
	if cfg.AgentServePort == 0 {
		cfg.AgentServePort = def.AgentServePort
	}
	if cfg.HostAgentBin == "" {
		cfg.HostAgentBin = def.HostAgentBin
	}
	// viper treats an empty env value as unset, so CELL_AGENT_URL="" would
	// silently fall back to the default instead of skipping the install.
	// Read it directly so the documented skip-install signal works.
	if raw, ok := os.LookupEnv("CELL_AGENT_URL"); ok {
		cfg.AgentURL = raw
	}
	return cfg, nil
}
