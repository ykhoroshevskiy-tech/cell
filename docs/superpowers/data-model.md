# Data Model: Cell Runtime

**Phase**: 1 (Design & Contracts)
**Date**: 2026-07-05

Normative for code generation. Field names, types, and enum values MUST be reproduced verbatim. Go struct tags.

## Enums

```go
type SessionState string

const (
    StateCreated   SessionState = "created"
    StateStaged    SessionState = "staged"
    StateDiskReady SessionState = "disk_ready"
    StateRunning   SessionState = "running"
    StateStopped   SessionState = "stopped"
    StateFailed    SessionState = "failed"
)
```

## Models

```go
package models

import "time"

type CellConfig struct {
    RuntimeRoot          string        `mapstructure:"runtime_root"`
    DataDir              string        `mapstructure:"data_dir"`
    ImagesDir            string        `mapstructure:"images_dir"`
    SessionDataDir       string        `mapstructure:"session_data_dir"`
    FirecrackerBin       string        `mapstructure:"firecracker_bin"`
    KernelPath           string        `mapstructure:"kernel_path"`
    RootfsPath           string        `mapstructure:"rootfs_path"`
    SquashfsPath         string        `mapstructure:"squashfs_path"`
    VCPUCount            int           `mapstructure:"vcpu_count"`
    MemSizeMiB           int           `mapstructure:"mem_size_mib"`
    ProjectDiskSizeMB    int           `mapstructure:"project_disk_size_mb"`
    BootTimeoutSec       time.Duration `mapstructure:"boot_timeout_sec"`
    SSHReadyTimeoutSec   time.Duration `mapstructure:"ssh_ready_timeout_sec"`
    GuestProjectMount    string        `mapstructure:"guest_project_mount"`
    GuestRepoDir         string        `mapstructure:"guest_repo_dir"`
    GuestAttachScript    string        `mapstructure:"guest_attach_script"`
    GuestProjectDevice   string        `mapstructure:"guest_project_device"`
    SSHUser              string        `mapstructure:"ssh_user"`
    TmuxSessionName      string        `mapstructure:"tmux_session_name"`
    IncludeGit           bool          `mapstructure:"include_git"`
    ExcludePatterns      []string      `mapstructure:"exclude_patterns"`
    AutoPull             bool          `mapstructure:"auto_pull"`
    AutoPullIntervalSec  int           `mapstructure:"auto_pull_interval_sec"`
    RebuildRootfs        bool          `mapstructure:"rebuild_rootfs"`
    SSHPublicKey         string        `mapstructure:"ssh_public_key"`
    CIPrefix             string        `mapstructure:"ci_prefix"`
    KernelVersion        string        `mapstructure:"kernel_version"`
    FirecrackerVersion   string        `mapstructure:"firecracker_version"`
    SquashfsVersion      string        `mapstructure:"squashfs_version"`
    AgentURL             string        `mapstructure:"agent_url"`
    AgentBin             string        `mapstructure:"agent_bin"`
    AgentCmd             string        `mapstructure:"agent_cmd"`
}

type NetworkConfig struct {
    TapName  string `json:"tap_name"`
    HostIP   string `json:"host_ip"`
    GuestIP  string `json:"guest_ip"`
    Netmask  string `json:"netmask"`
    GuestMac string `json:"guest_mac"`
    CIDR     int    `json:"cidr"`
}

type SessionRecord struct {
    SessionID        string         `json:"session_id"`
    RepoSource       string         `json:"repo_source"`
    CreatedAt        time.Time      `json:"created_at"`
    State            SessionState   `json:"state"`
    SessionDir       string         `json:"session_dir,omitempty"`
    StagedRepoDir    string         `json:"staged_repo_dir,omitempty"`
    ProjectDiskPath  string         `json:"project_disk_path,omitempty"`
    SocketPath       string         `json:"socket_path,omitempty"`
    SerialLogPath    string         `json:"serial_log_path,omitempty"`
    VmConfigPath     string         `json:"vm_config_path,omitempty"`
    FCPid            int            `json:"fc_pid,omitempty"`
    SSHKeyPath       string         `json:"ssh_key_path,omitempty"`
    SSHPublicKeyPath string         `json:"ssh_pubkey_path,omitempty"`
    NetworkConfig    *NetworkConfig `json:"network_config,omitempty"`
    Error            string         `json:"error,omitempty"`
}

type SessionStatus struct {
    SessionID    string `json:"session_id"`
    VMRunning    bool   `json:"vm_running"`
    SSHReachable bool   `json:"ssh_reachable"`
    TmuxReady    bool   `json:"tmux_ready"`
    RuntimeReady bool   `json:"runtime_ready"`
    GuestIP      string `json:"guest_ip,omitempty"`
    TapName      string `json:"tap_name,omitempty"`
}

type SessionListEntry struct {
    SessionID    string       `json:"session_id"`
    VMRunning    bool         `json:"vm_running"`
    SSHReachable bool         `json:"ssh_reachable"`
    TmuxReady    bool         `json:"tmux_ready"`
    RuntimeReady bool         `json:"runtime_ready"`
    GuestIP      string       `json:"guest_ip,omitempty"`
    TapName      string       `json:"tap_name,omitempty"`
    State        SessionState `json:"state"`
    RepoSource   string       `json:"repo_source"`
    CreatedAt    time.Time    `json:"created_at"`
}

type PullResult struct {
    Destination      string `json:"destination"`
    FilesTransferred *int   `json:"files_transferred,omitempty"`
}

type PullOptions struct {
    Dest    string `json:"dest,omitempty"`
    DryRun  bool   `json:"dry_run,omitempty"`
    Delete  bool   `json:"delete,omitempty"`
}
```

## Firecracker VM config models

```go
package models

type BootSourceSpec struct {
    KernelImagePath string `json:"kernel_image_path"`
    BootArgs        string `json:"boot_args"`
}

type DriveSpec struct {
    DriveID      string `json:"drive_id"`
    PathOnHost   string `json:"path_on_host"`
    IsRootDevice bool   `json:"is_root_device"`
    IsReadOnly   bool   `json:"is_read_only"`
}

type MachineConfigSpec struct {
    VCPUCount   int    `json:"vcpu_count"`
    MemSizeMiB  int    `json:"mem_size_mib"`
    SMT         bool   `json:"smt"`
    CPUTemplate string `json:"cpu_template"` // MUST be `"None"` when unset — Firecracker rejects `""`
}

type NetworkInterfaceSpec struct {
    IfaceID     string `json:"iface_id"`
    HostDevName string `json:"host_dev_name"`
    GuestMac    string `json:"guest_mac"`
}

type VmConfigDocument struct {
    BootSource        BootSourceSpec        `json:"boot_source"`
    Drives            []DriveSpec           `json:"drives"`
    MachineConfig     MachineConfigSpec     `json:"machine_config"`
    NetworkInterfaces []NetworkInterfaceSpec `json:"network_interfaces"`
}
```

## Network allocation algorithm

Given `session_id` (12 hex chars):

```text
octet = (int(session_id[:2], 16) % 250) + 1
tap_name = f"ctap-{session_id[:8]}"
host_ip = f"172.16.{octet}.1"
guest_ip = f"172.16.{octet}.2"
guest_mac = f"AA:FC:{session_id[0:2]}:{session_id[2:4]}:00:01"
cidr = 30
netmask = 255.255.255.252
```

Note: tap prefix changed from `ftap-` to `ctap-` (cell).

### Kernel boot args

`NetworkConfig.KernelBootArgs()` MUST return the full Firecracker `boot_args` string (matches Python `BootArgs.render()`):

```go
func (n *NetworkConfig) KernelBootArgs() string {
    return fmt.Sprintf(
        "console=ttyS0 reboot=k panic=1 pci=off init=/opt/guest-init/guest-entry.sh ip=%s::%s:%s::eth0:off",
        n.GuestIP, n.HostIP, n.Netmask,
    )
}
```

Session lifecycle MUST set `BootSource.boot_args` from `KernelBootArgs()` (not IP-only). Without `console=ttyS0` the kernel does not write to the Firecracker serial log and `waitReady` times out with no guest-init output.

`KernelIPArg()` MAY remain as a helper returning only the `ip=…` fragment; it MUST NOT be used alone as `boot_args`.

## Session artifact paths

For `session_id`, under `{session_data_dir}/{session_id}/`:

| File | Content |
|------|---------|
| `session.json` | `SessionRecord` JSON |
| `network.json` | `NetworkConfig` JSON |
| `vm-config.json` | Firecracker VM config document |
| `project.ext4` | Writable project disk |
| `repo/` | Staged copy before disk build |
| `serial.log` | Firecracker stdout/stderr |
| `id_ed25519` | Session SSH private key |
| `id_ed25519.pub` | Session SSH public key |
| `ssh-config` | Host-side SSH config snippet |
| `firecracker.socket` | API socket (runtime) |

## Bootstrap artifact paths

Under `{images_dir}/` (cached, content-verified by sha256):

| File | Source | Pinned in |
|------|--------|-----------|
| `vmlinux-<version>` | S3 `spec.ccfc.min` | `bootstrap/versions.go: Pinned[kernel]` |
| `bin/firecracker-<version>` | GitHub releases `.tgz`, extracted | `bootstrap/versions.go: Pinned[firecracker]` |
| `ubuntu-<version>.squashfs` | S3 `spec.ccfc.min` | `bootstrap/versions.go: Pinned[ubuntu-squashfs]` |
| `rootfs.ext4` | Built locally from squashfs (not downloaded) | n/a |

Symlinks `vmlinux` → `vmlinux-<version>`, `bin/firecracker` → `firecracker-<version>` are maintained by bootstrap for compatibility with `CellConfig` paths.

## Pinned versions table

`internal/bootstrap/versions.go` exports:

```go
type Artifact struct {
    Name    string
    Version string
    URL     string
    SHA256  string
}

var Pinned []Artifact
```

Bumping any artifact = one-line edit in `versions.go` (version + URL + sha256). No runtime "latest" resolution. See `contracts/bootstrap.md` for download algorithm.

## State transitions

```text
CREATED  → (prepare)
STAGED   → (stage)
DISK_READY → (build_disk)
RUNNING  → (start)
STOPPED  → (stop)
FAILED   → (unrecoverable error)
```

`start` allowed from `DISK_READY` or `STOPPED` (if pid dead). `stop` idempotent from any state.

## Environment variables (CELL_*)

Loaded via `viper.BindEnv` for each key (MUST bind, not rely on auto-env). All `CELL_*` keys are prefixed automatically.

| Variable | Maps to | Default |
|----------|---------|---------|
| `CELL_RUNTIME_ROOT` | `RuntimeRoot` | auto-detect from binary location |
| `CELL_DATA_DIR` | `DataDir` | `/var/lib/cell` |
| `CELL_PROJECT_DISK_SIZE_MB` | `ProjectDiskSizeMB` | 1024 |
| `CELL_VCPU_COUNT` | `VCPUCount` | 4 |
| `CELL_MEM_SIZE_MIB` | `MemSizeMiB` | 8192 |
| `CELL_INCLUDE_GIT` | `IncludeGit` | true |
| `CELL_SSH_USER` | `SSHUser` | agent |
| `CELL_TMUX_SESSION_NAME` | `TmuxSessionName` | agent |
| `CELL_AGENT_URL` | `AgentURL` | OpenCode tarball URL with `{target}` |
| `CELL_AGENT_BIN` | `AgentBin` | opencode |
| `CELL_AGENT_CMD` | `AgentCmd` | opencode --auto |
| `CELL_SSH_READY_TIMEOUT_SEC` | `SSHReadyTimeoutSec` | 90s |
| `CELL_BOOT_TIMEOUT_SEC` | `BootTimeoutSec` | 120s |
| `CELL_AUTO_PULL` | `AutoPull` | true |
| `CELL_AUTO_PULL_INTERVAL_SEC` | `AutoPullIntervalSec` | 30 |
| `CELL_REBUILD_ROOTFS` | `RebuildRootfs` | false |

## Guest environment (written by guest-entry.sh)

| Variable | Value |
|----------|-------|
| `TMPDIR` | `/tmp` |
| `BUN_TMPDIR` | `/tmp` |
| `npm_config_cache` | `/tmp/npm-cache` |
| `PATH` | `/usr/local/bin:/usr/bin:/bin` |
| `HOME` | `/home/agent` |
| `XDG_CACHE_HOME` | `$HOME/.cache` |
| `XDG_CONFIG_HOME` | `$HOME/.config` |

## Readiness predicates

| Field | True when |
|-------|-----------|
| `vm_running` | `FCPid` set and `os.FindProcess(pid); Signal(0)` succeeds |
| `ssh_reachable` | TCP connect guest:22 within timeout |
| `tmux_ready` | `ssh … tmux has-session -t {tmux_session_name}` exit 0 |
| `runtime_ready` | serial log contains substring `runtime ready` |

**Note**: `State` field is advisory only. `vm_running` is always computed via process liveness probe, never inferred from `State == running`. After host reboot or crash, sessions with stale `State=running` but dead pid MUST show `vm_running=false`.

**Note**: `State` field is advisory only. `vm_running` is always computed via process liveness probe, never inferred from `State == running`. After host reboot or crash, sessions with stale `State=running` but dead pid MUST show `vm_running=false`.

## CLI exit codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | Error (missing session, verify failed, stop partial failure) |
| 2 | Usage error (mutually exclusive flags, missing required flag) |
