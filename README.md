# cell

[![CI](https://github.com/ykhoroshevskiy-tech/cell/actions/workflows/ci.yml/badge.svg)](https://github.com/ykhoroshevskiy-tech/cell/actions/workflows/ci.yml)
![Go](https://img.shields.io/badge/go-1.26-00ADD8?logo=go&logoColor=white)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)]

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
[Security & threat model](#security--threat-model) ·
[Why not Docker / gVisor / Kata](#why-not-docker--gvisor--kata) ·
[Configuration](#configuration) ·
[Networking](#networking) ·
[Architecture](ARCHITECTURE.md) ·
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

Details in [ARCHITECTURE.md](ARCHITECTURE.md). The OpenCode TUI runs on your host (clipboard works locally, not over SSH). Exiting the TUI leaves the VM running; `sudo cell stop` shuts it down. After a host reboot, `sudo cell start --session <id>` boots the existing disk again.

## Demo

```sh
$ sudo cell launch --repo ~/projects/cell
✓ kernel cached (42.6 MiB)   ✓ firecracker cached (7.1 MiB)
✓ rootfs cached (4.0 GiB)    project disk 3072 MiB
session 5f2a9c1d3e07 ready at 172.16.107.2

$ cell ps
SESSION        STATE      GUEST_IP         REPO                      CREATED
5f2a9c2d3e07   running    172.16.107.2     /home/user/projects/cell  2026-09-13T12:00:00Z

$ sudo cell stop --all
stopped 1 session(s)
```

Shell completion in action — this is what zsh sees when you press TAB after
`sudo cell attach --session`:

```
$ sudo cell attach --session <TAB>
5f2a9c2d3e07  /home/user/projects/cell (running)
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

## Security & threat model

**What is protected:**

- **Host filesystem and processes** — the agent never runs on the host; it sees only the staged copy of your repo on the project disk (`/dev/vdb` inside the VM). No host mounts, no host socket, no Docker/container shared kernel.
- **Own kernel** — Firecracker microVMs boot a pinned kernel; agent-side kernel exploits land in the VM, not in your host.
- **Read-only rootfs** — the guest OS image is immutable; the agent can write only to `/tmp`, the project disk, and the `/usr/local` bind mount.
- **Guest-to-guest and guest-to-private isolation** — bridge port isolation blocks L2 traffic between guests; firewall drops guest→RFC1918 traffic. Guest egress is NATed through one bridge (`cell0`).
- **Guest API access** — the in-guest opencode server binds loopback and requires a password generated with `crypto/rand` (16 bytes hex, mode `0600`, root-owned in the host session dir).

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

## Configuration

A compact surface; the complete `CELL_*` reference lives in
[CONFIGURATION.md](CONFIGURATION.md).

| Setting | Default |
|---------|---------|
| `CELL_DATA_DIR` | `/var/lib/cell` |
| `CELL_PROJECT_DISK_SIZE_MB` | `3072` |
| `CELL_VCPU_COUNT` / `CELL_MEM_SIZE_MIB` | `4` / `8192` |
| `CELL_AUTO_PULL` / `CELL_AUTO_PULL_INTERVAL_SEC` | `true` / `30` |
| `CELL_INSTALL_SUPERPOWERS` | `false` (opt-in guest agent tooling) |

## Networking

All sessions share one Linux bridge (`cell0`, `172.16.107.1/24`) with stable guest IPs (`.2`–`.254`) persisted per session. Bridge port isolation blocks guest-to-guest traffic; the runtime reconciles bridge/TAP/firewall state under a global lock before any mutating command. Full details in [ARCHITECTURE.md](ARCHITECTURE.md#networking).

Legacy sessions (pre-bridge network records) are not migrated. Remove them with `sudo cell rm --session <id> --legacy`, then launch again.

## License

MIT © Yuri Khoroshevskiy
