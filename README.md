# cell

[![CI](https://github.com/ykhoroshevskiy-tech/cell/actions/workflows/ci.yml/badge.svg)](https://github.com/ykhoroshevskiy-tech/cell/actions/workflows/ci.yml)
[![Coverage](https://codecov.io/gh/ykhoroshevskiy-tech/cell/graph/badge.svg)](https://codecov.io/gh/ykhoroshevskiy-tech/cell)
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

## Contents

[Why cell](#why-cell) ·
[How it works](#how-it-works) ·
[Demo](#demo) ·
[Requirements](#requirements) ·
[Build & install](#build--install) ·
[Quick start](#quick-start) ·
[Commands](#commands) ·
[Architecture](#architecture) ·
[Configuration](#configuration) ·
[Networking](#networking) ·
[Security & threat model](#security--threat-model) ·
[Why not Docker / gVisor / Kata](#why-not-docker--gvisor--kata) ·
[Roadmap](#roadmap) ·
[License](#license)

## Why cell

- **Agent isolation** — the agent runs in its own kernel (Firecracker/KVM), not as a process or container sharing your host kernel and home directory.
- **Bounded blast radius** — broken deps, runaway installs, and destructive commands stay in the VM.
- **Your work comes back** — guest workspace changes sync to the host repo (rsync / auto-pull), so isolation isn’t a dead end.

## How it works

1. **Stage** — the repo is copied onto a fresh project disk image
2. **Boot** — Firecracker microVM with a pinned kernel and read-only rootfs
3. **Serve** — `opencode serve` runs in the guest (loopback, basic-auth)
4. **Attach** — the host TUI connects over an SSH `-L` tunnel
5. **Sync** — auto-pull rsyncs guest changes back to the host repo

The OpenCode TUI runs on your host (clipboard works locally, not over SSH). Exiting the TUI leaves the VM running; `sudo cell stop` shuts it down. After a host reboot, `sudo cell start --session <id>` boots the existing disk again.

## Demo

```sh
$ sudo cell launch --repo ~/projects/cell
✓ kernel cached (42.6 MiB)   ✓ firecracker cached (7.1 MiB)
✓ rootfs cached (4.0 GiB)    project disk 3072 MiB
session 5f2a9c1d3e07 ready at 172.16.107.2

$ cell ps
SESSION        STATE      GUEST_IP         REPO                      CREATED
5f2a9c1d3e07   running    172.16.107.2     /home/user/projects/cell  2026-09-13T12:00:00Z

$ sudo cell stop --all
stopped 1 session(s)
```

Shell completion in action — this is what zsh sees when you press TAB after
`sudo cell attach --session`:

```
$ sudo cell attach --session <TAB>
5f2a9c1d3e07  /home/user/projects/cell (running)
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

`--config` copies the file into `.filter/opencode.json`; the guest loads it as the agent config. It is not stored in `session.json` and is skipped by pull.

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

## Architecture

### Lifecycle

```
prepare            boot                serve                attach             sync
repo → copy     →  mkfs project   →  firecracker VM  →   agent serve    →  ssh -L tunnel
   staged tree      disk image         kernel+rootfs+disk    (in-guest)         host TUI
```

1. **Prepare session** — a random 12-hex session id; per-session directory under `<data>/session-data/<id>/` holds `session.json`, `network.json`, `vm-config.json`, an SSH keypair (`id_ed25519`, 0600), `serial.log`, `project.ext4`, and the `firecracker.socket`.
2. **Stage** — the repo is copied (rsync, excludes `__pycache__`, `node_modules`, `.venv`, `images`, `session-data`) into a temp root together with `.filter/` (`authorized_keys`, serve password), then written into a fresh ext4 image via `mkfs.ext4 -d` — that image is the project disk.
3. **Boot** — Firecracker starts with the pinned kernel, a read-only Ubuntu 24.04 rootfs (debootstrap), and the project disk as `/dev/vdb`. The guest-init script mounts `/project` and `/usr/local` from the project disk and starts the agent command in tmux.
4. **Serve** — the agent runs in the guest; the in-guest server binds loopback and is protected by basic auth. The random password (`crypto/rand`, 16 bytes hex) lives in the session dir and on the disk root (`.filter/opencode-server.pass`).
5. **Wait ready** — readiness = VM alive + SSH port open + server health probe (`curl` in the guest over loopback with the password). A failing boot streams the serial log.
6. **Attach** — the host picks a free local port, starts `ssh -N -L <port>:127.0.0.1:<serve-port>`, then runs the host TUI (`opencode attach http://127.0.0.1:<port> --dir /project --continue -p <password>`) in the foreground. The TUI runs on the host; the agent runs in the VM.
7. **Sync** — auto-pull (default every 30s while attached) rsyncs the guest workspace back to the host repo over SSH (`-a --no-owner --no-group --chown=<host owner>`; `--delete` only on explicit `--delete`). `.filter` is excluded both ways.

### Privilege model

Read-only commands (`ps`, `logs`, `version`, `help`, `completion`, `__complete`) only read world-readable state and run without root. Every mutating command requires root and exits with `cell: <cmd> requires root — run: sudo cell <cmd>` otherwise (central gate in `internal/cli/root.go`).

VM liveness is computed from `/proc/<pid>/cmdline` matched against the session API socket — world-readable, so `ps` stays truthful for root-owned VMs without root.

### Bootstrap artifacts

`sudo cell bootstrap` pins and caches into `<data>/images/`:

- `vmlinux-<kernel pin>` (symlink `vmlinux`) — Firecracker CI kernel
- `bin/firecracker-<version>` (symlink `bin/firecracker`) + `jailer` from the same release
- `rootfs.ext4` — built via `debootstrap noble --variant=minbase`, guest packages (`openssh-server tmux zsh rsync curl git sudo ca-certificates`), Node LTS + uv + CPython into `/usr/local` (tarballs pinned by version), sshd config, an `agent` user with passwordless sudo, and embedded guest-init scripts written into the rootfs at build time
- optional Superpowers install (opt-in, `CELL_INSTALL_SUPERPOWERS`)

A stamp file records the build inputs; changing any pin or the superpowers flag rebuilds the rootfs.

## Configuration

All settings are overridden with `CELL_*` environment variables. Defaults are fixed for reproducible builds.

### Runtime

| Setting | Default |
|---------|---------|
| `CELL_DATA_DIR` | `/var/lib/cell` |
| `CELL_PROJECT_DISK_SIZE_MB` | `3072` (rootfs stays 4096 MiB) |
| `CELL_VCPU_COUNT` / `CELL_MEM_SIZE_MIB` | `4` / `8192` |
| `CELL_AUTO_PULL` / `CELL_AUTO_PULL_INTERVAL_SEC` | `true` / `30` |
| `CELL_INSTALL_SUPERPOWERS` | `false` (opt-in guest agent tooling) |

### Artifact pins

| Setting | Default |
|---------|---------|
| `CELL_CI_PREFIX` | `firecracker-ci/20260708-f11c230ed107-0/` |
| `CELL_KERNEL_VERSION` | `6.1.176` |
| `CELL_FIRECRACKER_VERSION` | `v1.16.1` |
| `CELL_NODE_VERSION` / `CELL_UV_VERSION` / `CELL_PYTHON_VERSION` | `v24.20.0` / `0.12.7` / `3.13` |

Path overrides (`CELL_KERNEL_PATH`, `CELL_FIRECRACKER_BIN`, `CELL_ROOTFS_PATH`) still win when the file exists. A rootfs rebuild is triggered when any pin changes (`sudo cell bootstrap --rebuild-rootfs`; the host needs the `debootstrap` package).

### In-guest agent (vendor-neutral; defaults install OpenCode)

| Setting | Default |
|---------|---------|
| `CELL_AGENT_URL` | OpenCode release tarball (`…/opencode-{target}.tar.gz`); empty skips install |
| `CELL_AGENT_BIN` | `opencode` |
| `CELL_AGENT_CMD` | `opencode serve --hostname 127.0.0.1 --port 4096` |
| `CELL_AGENT_SERVE_PORT` | `4096` |
| `CELL_HOST_AGENT_BIN` | `opencode` |
| `CELL_TMUX_SESSION_NAME` | `agent` |

`{target}` in the URL is replaced with `linux-x64-baseline` / `linux-arm64-baseline`. The host must have OpenCode installed (`CELL_HOST_AGENT_BIN` or `opencode` on PATH); `sudo cell launch` / `sudo cell attach` fail fast if it is missing — the VM keeps running.

The guest rootfs is read-only; `/usr/local` is bind-mounted from the project disk so the agent can install to `/usr/local/bin`. The guest has `git` and passwordless `sudo` for user `agent`.

## Networking

All sessions share one Linux bridge (`cell0`, `172.16.107.1/24`) with stable guest IPs (`.2`–`.254`) persisted per session (`network_version: 2`).

- Per session a TAP device (`ctap-<id>`) is created and enslaved to `cell0` with **no host-side IP**; bridge **port isolation** blocks guest-to-guest L2 traffic.
- iptables chains (`CELL_INPUT`, `CELL_FORWARD`, `CELL_NAT`) are created idempotently: established/related accepted, host `:8080` accepted, guest-sourced input dropped, guest→private CIDR and guest→guest forwarding dropped, subnet egress accepted + one MASQUERADE rule, `ip_forward=1`.
- Before `launch`, `start`, `stop`, and `rm` the runtime converges bridge/TAP/firewall state to the live session set under a global lock (`/run/lock/cell-network.lock`, `flock`): stale TAPs are removed, live sessions re-attached, dead processes repaired in `session.json`.
- Legacy sessions (pre-bridge network records) are not migrated. Remove them with `sudo cell rm --session <id> --legacy`, then launch again.

## Security & threat model

**What is protected:**

- **Host filesystem and processes** — the agent never runs on the host; it sees only the staged copy of your repo on the project disk (`/dev/vdb` inside the VM). No host mounts, no host socket, no Docker/container shared kernel.
- **Own kernel** — Firecracker microVMs boot a pinned kernel; agent-side kernel exploits land in the VM, not in your host.
- **Read-only rootfs** — the guest OS image is immutable; the agent can write only to `/tmp`, the project disk, and the `/usr/local` bind mount.
- **Guest-to-guest and guest-to-private isolation** — bridge port isolation blocks L2 traffic between guests; firewall drops guest→RFC1918 traffic. Guest egress is NATed through one bridge (`cell0`).
- **Guest API access** — the in-guest agent server binds loopback and requires a password generated with `crypto/rand` (16 bytes hex, mode `0600`, root-owned in the host session dir).

**What is NOT protected (read this):**

- **Unrestricted guest egress** — guests reach the internet through NAT; there is no per-guest egress policy. A compromised agent can call any external host.
- **Host commands run as root** — every mutating `cell` command executes on the host with full root privileges. `cell` is trusted tooling; do not grant sudo to untrusted users.
- **No Firecracker jailer** — the VMM runs as root without `--jailer` sandboxing; the isolation boundary is the KVM/VM boundary itself.
- **The password is readable inside the guest** — `.filter/opencode-server.pass` sits in the agent’s own workspace; the host↔VM boundary is what it protects, not agent↔guest.
- **The guest trusts its `agent` user** — passwordless sudo inside the guest is by design (the agent IS the admin of the VM); host safety comes from the VM boundary, not guest users.

## Why not Docker / gVisor / Kata

- **Docker / containers** — same host kernel, isolation by namespaces and cgroups. Kernel attacks, `/proc`/`/sys` leakage, and container-escape CVEs apply directly to the host. `cell` gives every agent its own kernel with a minimal attack surface (KVM + virtio only).
- **gVisor** — sandboxes syscalls in a userspace kernel. Strong syscall boundary, but a translation layer with real performance costs and syscall gaps — and it still mediates against the host kernel, not a separate one. Firecracker keeps full syscall compatibility with hardware isolation.
- **Kata Containers** — also full VMs, but carries the whole container stack (QEMU, runtimes). Firecracker is a purpose-built microVMM with sub-second boot; `cell` adds the agent workflow (staging, tmux, tunnel, workspace sync) on top.
- **The cost:** Firecracker needs `/dev/kvm` and a Linux host — it does not run inside VMs without nested virtualization, and not on macOS/Windows.

## Roadmap

- **Egress policy** — per-guest outbound allowlist / policy proxy; today guests have unrestricted internet via NAT (see threat model). This is the biggest remaining hole.
- **Rootless VMM** — run Firecracker under the `jailer` (already fetched by bootstrap) or an unprivileged VMM user instead of root.
- **More agents** — second in-guest agent (Claude Code) over the vendor-neutral `CELL_AGENT_*` layer; validate, don’t just claim.
- **Portfolio polish** — release binaries with tags (`v0.1.0`), asciinema demo.

## License

[MIT](LICENSE) © Yuri Khoroshevskiy
