# cell

A CLI manager for [OpenCode](https://github.com/sst/opencode) sessions running inside [Firecracker](https://github.com/firecracker-microvm/firecracker) microVMs.

`cell` stages a source repository onto a project disk, boots a Firecracker microVM with a pinned kernel/rootfs, attaches via SSH into a tmux session running OpenCode, and continuously rsyncs the guest workspace back to the host.

## Requirements

- **Linux with KVM** (`/dev/kvm`). Other platforms are not supported.
- **Root.** `cell` must run as root (`sudo cell`); `version`/`help` are the only exceptions.
- **Go 1.26+** to build.
- `curl`, `tar`, `mkfs.ext4`, `ssh`, `rsync`, `jq` on the host.

## Build

```sh
go build -o cell ./cmd/cell
```

## Quick start

```sh
# 1. Download/build the kernel, rootfs, and firecracker binary
sudo ./cell bootstrap

# 2. Launch a repo into a microVM and attach
sudo ./cell launch --repo /path/to/your/repo

# 3. List running sessions
sudo ./cell ps

# 4. Reattach to a session's SSH + tmux
sudo ./cell ssh <session-id>

# 5. Pull the guest workspace back to the host repo
sudo ./cell pull <session-id>

# 6. Stop one (or all) sessions
sudo ./cell stop <session-id>
sudo ./cell stop --all
```

## Commands

| Command     | Description                                         |
|-------------|-----------------------------------------------------|
| `bootstrap` | Download/build kernel, rootfs, firecracker          |
| `launch`    | Stage repo, boot VM, attach SSH, auto-pull          |
| `stop`      | Stop one or all VMs                                 |
| `ssh`       | Reattach SSH + tmux                                 |
| `status`    | Probe VM/SSH/tmux for one session                  |
| `verify`    | Readiness check with serial tail on failure         |
| `logs`      | Print serial.log                                    |
| `ps`        | List sessions                                       |
| `pull`      | Rsync guest workspace back to host repo             |
| `rescue`    | Extract workspace from project disk                 |
| `version`   | Print version                                       |

Global flags: `--quiet`, `--verbose` (`-v`).

## Configuration

All defaults can be overridden with `CELL_*` environment variables (e.g. `CELL_VCPU_COUNT`, `CELL_MEM_SIZE_MIB`, `CELL_DATA_DIR`). Key defaults:

| Setting                 | Default                          |
|-------------------------|----------------------------------|
| `data_dir`              | `/var/lib/cell`                  |
| `images_dir`            | `<data_dir>/images`              |
| `session_data_dir`      | `<data_dir>/session-data`        |
| `vcpu_count`            | `4`                              |
| `mem_size_mib`          | `8192`                           |
| `project_disk_size_mb`  | `1024`                           |
| `boot_timeout_sec`      | `120s`                           |
| `ssh_ready_timeout_sec` | `90s`                            |
| `ssh_user`              | `agent`                          |
| `tmux_session_name`     | `opencode`                       |
| `auto_pull`             | `true`                           |
| `auto_pull_interval_sec`| `30`                             |

## Project layout

```
cmd/cell/          entry point
internal/cli/      cobra commands
internal/bootstrap artifact download/build + version resolution
internal/hypervisor firecracker process management
internal/firecracker firecracker API client
internal/session   VM lifecycle
internal/network   TAP/interface setup
internal/ssh        key generation + attach
internal/sync       rsync pull / auto-pull
internal/stage      repo staging onto project disk
internal/models     shared types
internal/config     viper-based config
internal/verbose    verbose logging flag
guestinit/          embedded shell scripts run inside the guest
scripts/            host-side helper scripts
```

## License

[MIT](LICENSE) © Yuriy Khoroshevskiy
