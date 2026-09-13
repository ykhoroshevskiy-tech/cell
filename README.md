# cell

[![CI](https://github.com/ykhoroshevskiy-tech/cell/actions/workflows/ci.yml/badge.svg)](https://github.com/ykhoroshevskiy-tech/cell/actions/workflows/ci.yml)
![Go](https://img.shields.io/badge/go-1.26-00ADD8?logo=go&logoColor=white)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

Run a coding agent inside a Firecracker microVM — not on your host.

AI coding agents need shell access, package installs, and freedom to change files. On the host that means a large blast radius. `cell` boots a lightweight KVM microVM, puts your repo inside it, and keeps the agent’s filesystem, processes, and network away from your machine — while syncing useful work back.

```
 ┌──────────────────────────── host ────────────────────────────┐
 │                                                              │
 │  sudo cell launch ───▶ firecracker ───▶ microVM              │
 │       │                                       │              │
 │       │                    ┌──────────────────┴────────────┐ │
 │   opencode TUI ◀─ssh -L─── │ kernel + rootfs (ro)          │ │
 │        (your clipboard)    │ /project: your repo (rw)      │ │
 │        ▲                   │ opencode serve :4096          │ │
 │        │  auto-pull rsync  │ node/uv/python installed      │ │
 │        ▼                   └───────────────┬───────────────┘ │
 │   host repo ◀──────────────────────────── cell0 bridge + NAT  │
 └──────────────────────────────────────────────────────────────┘
```

## Why cell

- **Agent isolation** — the agent runs in its own kernel (Firecracker/KVM), not as a process or container sharing your host kernel and home directory.
- **Bounded blast radius** — broken deps, runaway installs, and destructive commands stay in the VM.
- **Your work comes back** — guest workspace changes sync to the host repo (rsync / auto-pull), so isolation isn’t a dead end.

## How it works

1. **Stage** — copy the repo onto a project disk (includes `.filter/opencode-server.pass`)
2. **Boot** — Firecracker microVM with a pinned kernel/rootfs
3. **Serve** — `opencode serve` runs in the guest (binds `127.0.0.1`)
4. **Attach** — host runs `opencode attach` over an SSH `-L` tunnel to the guest server
5. **Sync** — `cell pull` or auto-pull while attached syncs guest workspace changes back to the host repo (skips `.filter`). `--repo` is stored as an absolute path; pull dest must be absolute. Optional `--config` is copied to `.filter/opencode.json` and loaded as the guest agent config (`~/.config/opencode/opencode.json`); it is not stored in `session.json` and is skipped by pull.

The OpenCode TUI runs on your host (clipboard works locally, not over SSH). Exiting the TUI leaves the VM running; use `sudo cell stop` to shut it down. After a host reboot, `sudo cell start --session <id>` boots the existing disk again.

You keep working as if the agent is local; the risky part stays in the VM.

## Demo

```sh
$ sudo cell launch --repo ~/projects/cell
✓ kernel cached (42.6 MiB)   ✓ firecracker cached (7.1 MiB)
✓ rootfs cached (4.0 GiB)    project disk 3072 MiB
session 5f2a9c1d3e07 ready at 172.16.107.2

$ cell ps
SESSION        STATE      GUEST_IP         REPO                      CREATED
5f2a9c2d3e07   running    172.16.107.2     /home/user/projects/cell  2026-09-13T12:00:00Z

$ cell __complete attach --session ''
5f2a9c2d3e07	/home/user/projects/cell (running)
:4

$ sudo cell stop --all
stopped 1 session(s)
```

Shell completions (`cell completion zsh`) and the oh-my-zsh plugin
([`plugins/cell`](plugins/cell/README.md)) add short aliases — `ca`, `cl`, `cps`,
`cstop` — with live session completion.

## Status

Early / experimental.

**Tested so far only with [OpenCode](https://github.com/sst/opencode)** as the in-guest coding agent. Other agents may work later; they are not validated yet.

Requires Linux with KVM. Mutating commands (`bootstrap`, `launch`, `start`, `stop`, `rm`, `attach`, `ssh`, `pull`, `status`, `verify`, `rescue`) must run as root: run them with `sudo`; without root they fail with `cell: <cmd> requires root — run: sudo cell <cmd>`. Read-only commands (`ps`, `logs`, `version`, `help`) work without root.

## Requirements

- Linux with KVM (`/dev/kvm`)
- Root (`sudo`) for all mutating commands; `cell ps` and `cell logs` work without root
- Go 1.26+ to build
- Host tools: `curl`, `tar`, `mkfs.ext4`, `ssh`, `rsync`, `debootstrap`, [OpenCode](https://github.com/sst/opencode) (`opencode` on PATH)

## Build & install

Version bumps on every commit automatically: `scripts/build.sh` derives
`0.<minor>.<commit-count>+g<short-sha>` from git history and embeds it via
`-ldflags`.

```sh
scripts/build.sh            # produces ./cell with the git-derived version
sudo install -m 755 cell /usr/bin/cell
sudo cell bootstrap   # downloads/builds kernel, rootfs, firecracker into /var/lib/cell
```

Re-run `sudo cell bootstrap` after reinstalling the binary.

Optional network smoke test (KVM, prior bootstrap):

```sh
sudo scripts/e2e-network.sh ./cell
```

## Quick start

```sh
sudo cell bootstrap
sudo cell launch --repo /path/to/your/repo
sudo cell launch --repo /path/to/your/repo --config /path/to/opencode.json   # optional

cell ps
sudo cell attach --session <session-id>
sudo cell pull --session <session-id>
sudo cell stop --session <session-id>
```

## Commands

| Command     | Description                                   |
|-------------|-----------------------------------------------|
| `bootstrap` | Download/build kernel, rootfs, firecracker    |
| `launch`    | Stage repo, boot VM, attach host TUI, auto-pull |
| `start`     | Boot an existing session disk (e.g. after reboot) |
| `attach`    | Reconnect host TUI to a running session       |
| `stop`      | Stop one or all VMs                           |
| `rm`        | Remove a stopped session and free its guest IP |
| `ssh`       | Debug SSH + tmux (serve logs)                 |
| `status`    | Probe VM/SSH/opencode server for one session  |
| `verify`    | Readiness check (server, not tmux) with serial tail on failure |
| `logs`      | Print serial.log                              |
| `ps`        | List sessions                                 |
| `pull`      | Rsync guest workspace back to host repo       |
| `rescue`    | Extract workspace from project disk (needs sudo) |
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
| `CELL_SQUASHFS_VERSION` | unused (rootfs is debootstrap noble, not squashfs) |

Path overrides (`CELL_KERNEL_PATH`, `CELL_FIRECRACKER_BIN`, `CELL_SQUASHFS_PATH`) still win when the file already exists.

In-guest agent (vendor-neutral; **defaults install OpenCode**):

| Setting | Default |
|---------|---------|
| `CELL_AGENT_URL` | OpenCode release tarball (`…/opencode-{target}.tar.gz`); empty skips install |
| `CELL_AGENT_BIN` | `opencode` |
| `CELL_AGENT_CMD` | `opencode serve --hostname 127.0.0.1 --port 4096` |
| `CELL_AGENT_SERVE_PORT` | `4096` |
| `CELL_HOST_AGENT_BIN` | `opencode` |
| `CELL_TMUX_SESSION_NAME` | `agent` |

`{target}` in the URL is replaced with `linux-x64-baseline` / `linux-arm64-baseline`.

The host must have OpenCode installed (`CELL_HOST_AGENT_BIN` or `opencode` on PATH). `cell launch` / `cell attach` fail fast if it is missing; the VM keeps running.

Guest rootfs is Ubuntu 24.04 via `debootstrap` (apt works) and is mounted read-only. `/usr/local` is bind-mounted from the project disk so the agent can install to `/usr/local/bin`. Node LTS (`CELL_NODE_VERSION`, default `v24.20.0`), uv (`CELL_UV_VERSION`, default `0.12.7`), and CPython (`CELL_PYTHON_VERSION`, default `3.13`) are installed to `/usr/local`. Superpowers is installed at `/opt/opencode-plugins`. Rebuild after this change: `sudo cell bootstrap --rebuild-rootfs`. Host needs the `debootstrap` package.

The guest has `git` and passwordless `sudo` for user `agent`. New sessions get a 3072 MiB project disk (`CELL_PROJECT_DISK_SIZE_MB`); rootfs size stays 4096 MiB (`CELL_ROOTFS_SIZE_MB`) and does not scale with the project disk. Existing session disks are not resized.

Runtime:

| Setting                  | Default           |
|--------------------------|-------------------|
| `CELL_DATA_DIR`          | `/var/lib/cell`   |
| `CELL_PROJECT_DISK_SIZE_MB` | `3072`         |
| `CELL_ROOTFS_SIZE_MB`    | `4096`            |
| `CELL_NODE_VERSION`      | `v24.20.0`        |
| `CELL_UV_VERSION`        | `0.12.7`          |
| `CELL_PYTHON_VERSION`    | `3.13`            |
| `CELL_VCPU_COUNT`        | `4`               |
| `CELL_MEM_SIZE_MIB`      | `8192`            |
| `CELL_AUTO_PULL`         | `true`            |
| `CELL_AUTO_PULL_INTERVAL_SEC` | `30`         |

## Networking

All sessions share one Linux bridge (`cell0`, `172.16.107.1/24`). Each session gets a stable guest IP (`.2`–`.254`) persisted in `session.json` (`network_version: 2`). TAP devices attach to the bridge without host-side IP addresses; bridge port isolation blocks guest-to-guest L2 traffic.

Before `launch`, `start`, `stop`, and `rm`, cell reconciles bridge/TAP/firewall state under `/run/lock/cell-network.lock`. Stopped sessions keep their IP lease until removed.

Legacy sessions (pre-bridge network records) are not migrated. Remove them with `sudo cell rm --session <id> --legacy`, then launch again.

After building, `sudo scripts/e2e-network.sh ./cell` smoke-tests two VMs (distinct IPs, guest isolation, stale TAP cleanup, lease reuse). Host reboot recovery (`cell start`) is not automated in that script.

## License

[MIT](LICENSE) © Yuri Khoroshevskiy
