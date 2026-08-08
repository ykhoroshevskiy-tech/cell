# cell

Run a coding agent inside a Firecracker microVM — not on your host.

AI coding agents need shell access, package installs, and freedom to change files. On the host that means a large blast radius. `cell` boots a lightweight KVM microVM, puts your repo inside it, and keeps the agent’s filesystem, processes, and network away from your machine — while syncing useful work back.

## Why cell

- **Agent isolation** — the agent runs in its own kernel (Firecracker/KVM), not as a process or container sharing your host kernel and home directory.
- **Bounded blast radius** — broken deps, runaway installs, and destructive commands stay in the VM.
- **Your work comes back** — guest workspace changes sync to the host repo (rsync / auto-pull), so isolation isn’t a dead end.

## How it works

1. **Stage** — copy the repo onto a project disk
2. **Boot** — start a Firecracker microVM with a pinned kernel/rootfs
3. **Attach** — SSH into a tmux session where the agent runs
4. **Sync** — pull guest workspace changes back to the host repo

You keep working as if the agent is local; the risky part stays in the VM.

## Status

Early / experimental.

**Tested so far only with [OpenCode](https://github.com/sst/opencode)** as the in-guest coding agent. Other agents may work later; they are not validated yet.

Requires Linux with KVM. Runtime commands must run as root (`sudo cell`); `version` and `help` do not.

## Requirements

- Linux with KVM (`/dev/kvm`)
- Root for runtime commands
- Go 1.26+ to build
- Host tools: `curl`, `tar`, `mkfs.ext4`, `ssh`, `rsync`

## Build & install

```sh
go build -o cell ./cmd/cell
sudo install -m 755 cell /usr/bin/cell
```

## Quick start

```sh
sudo cell bootstrap
sudo cell launch --repo /path/to/your/repo

sudo cell ps
sudo cell ssh <session-id>
sudo cell pull <session-id>
sudo cell stop <session-id>
```

## Commands

| Command     | Description                                   |
|-------------|-----------------------------------------------|
| `bootstrap` | Download/build kernel, rootfs, firecracker    |
| `launch`    | Stage repo, boot VM, attach SSH, auto-pull    |
| `stop`      | Stop one or all VMs                           |
| `ssh`       | Reattach SSH + tmux                           |
| `status`    | Probe VM/SSH/tmux for one session             |
| `verify`    | Readiness check with serial tail on failure   |
| `logs`      | Print serial.log                              |
| `ps`        | List sessions                                 |
| `pull`      | Rsync guest workspace back to host repo       |
| `rescue`    | Extract workspace from project disk           |
| `version`   | Print version                                 |

Global flags: `--quiet`, `--verbose` (`-v`).

## Configuration

Defaults can be overridden with `CELL_*` environment variables.

Artifact pins (defaults are fixed for reproducible bootstrap; override to change the stack):

| Setting | Default |
|---------|---------|
| `CELL_CI_PREFIX` | `firecracker-ci/20260708-f11c230ed107-0/` |
| `CELL_KERNEL_VERSION` | `6.1.176` |
| `CELL_FIRECRACKER_VERSION` | `v1.16.1` |
| `CELL_SQUASHFS_VERSION` | `24.04` |

Path overrides (`CELL_KERNEL_PATH`, `CELL_FIRECRACKER_BIN`, `CELL_SQUASHFS_PATH`) still win when the file already exists.

Runtime:

| Setting                  | Default           |
|--------------------------|-------------------|
| `CELL_DATA_DIR`          | `/var/lib/cell`   |
| `CELL_VCPU_COUNT`        | `4`               |
| `CELL_MEM_SIZE_MIB`      | `8192`            |
| `CELL_AUTO_PULL`         | `true`            |
| `CELL_AUTO_PULL_INTERVAL_SEC` | `30`         |

## License

[MIT](LICENSE) © Yuri Khoroshevskiy
