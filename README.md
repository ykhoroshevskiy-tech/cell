# cell

[![CI](https://github.com/ykhoroshevskiy-tech/cell/actions/workflows/ci.yml/badge.svg)](https://github.com/ykhoroshevskiy-tech/cell/actions/workflows/ci.yml)
![Go](https://img.shields.io/badge/go-1.26-00ADD8?logo=go&logoColor=white)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

Run a coding agent inside a Firecracker microVM — not on your host.

Agents need shell, packages, and file freedom; on the host that's your `~/.ssh`. `cell` boots a KVM microVM, puts your repo inside it, and keeps the agent's filesystem, processes, and network away from your machine — while rsyncing useful work back.

```
sudo cell launch ──▶ firecracker ──▶ microVM (own kernel, ro rootfs)
      │                                  │ repo on /dev/vdb (rw)
host TUI ◀── ssh -L tunnel ──│ opencode serve :4096
host repo ◀── auto rsync ◀───┤ node/uv/python in guest
                             cell0 bridge: port isolation + NAT
```

**Validated with [OpenCode](https://github.com/sst/opencode) (tunnelled host TUI, clipboard stays local) and [Claude Code](https://github.com/anthropics/claude-code) (TUI over SSH+tmux inside the VM).** Unlike containers/gVisor, the agent gets its own kernel — host kernel exploits and container escapes don't apply. Costs: needs `/dev/kvm`, Linux only.

## Install & quick start

```sh
scripts/build.sh && sudo install -m 755 cell /usr/bin/cell
sudo cell bootstrap       # kernel, rootfs, firecracker → /var/lib/cell
sudo cell launch --agent claude --repo /path/to/repo   # or --agent opencode
cell ps                   # session ids work without sudo
sudo cell attach --session <id>    # reconnect TUI
sudo cell stop --all
```

Mutating commands need root (`sudo`); without it they fail with `cell: <cmd> requires root — run: sudo cell <cmd>`. Exiting the TUI keeps the VM running; after host reboot: `sudo cell start --session <id>`.

## Commands

| Command     | Description                                   |
|-------------|-----------------------------------------------|
| `bootstrap` | Download/build kernel, rootfs, firecracker    |
| `launch`    | Stage repo, boot VM, attach (`--agent opencode\|claude\|none`) |
| `start` / `attach` | Boot existing session / reconnect TUI |
| `stop` / `rm` | Stop one (`--session`) or all (`--all`); `rm` frees disk+IP |
| `ps` / `logs` / `status` / `verify` | List, serial log, per-session probes |
| `ssh`       | Debug shell + tmux in the guest               |
| `pull`      | Rsync guest workspace → host repo             |
| `rescue`    | Extract workspace from the project disk       |

## Security & threat model

**Protected:** agent never runs on the host — it sees only a staged repo copy on its own disk; own kernel via KVM; read-only rootfs; guest↔guest and guest→private-network traffic dropped (bridge port isolation + iptables); in-guest server binds loopback with a `crypto/rand` password, `0600`, root-owned.

**NOT protected — read this before trusting it:**

- **Unrestricted guest egress** — NAT to the full internet; no per-guest policy. A compromised agent can upload your repo and keys.
- **Host commands run as root** — `cell` is trusted tooling; do not grant sudo to untrusted users.
- **No Firecracker jailer** — VMM runs as root; the boundary is KVM/VM itself.
- **The password is readable inside the guest**, and guest `agent` has passwordless sudo by design (the agent administers its VM; the VM boundary is the protection).

## Configuration (env)

| Setting | Default |
|---------|---------|
| `CELL_AGENT` | `opencode` (`claude` = TUI in-guest via tmux; `none` = SSH-only) |
| `CELL_DATA_DIR` | `/var/lib/cell` |
| `CELL_PROJECT_DISK_SIZE_MB` | `3072` |
| `CELL_VCPU_COUNT` / `CELL_MEM_SIZE_MIB` | `4` / `8192` |
| `CELL_AUTO_PULL` / `CELL_AUTO_PULL_INTERVAL_SEC` | `true` / `30` |

Pins (`CELL_KERNEL_VERSION`, `CELL_FIRECRACKER_VERSION`, guest node/uv/python) default to fixed versions; `sudo cell bootstrap --rebuild-rootfs` applies changes. Host needs: `curl`, `tar`, `mkfs.ext4`, `ssh`, `rsync`, `debootstrap`, Go 1.26+ to build, and [OpenCode](https://github.com/sst/opencode) for the tunnelled-TUI path.

## Roadmap

- Per-guest egress policy (allowlist/proxy) — the biggest remaining hole above.
- Rootless VMM via the fetched `jailer`.
- CI release job for tagged binaries.

## License

[MIT](LICENSE) © Yuri Khoroshevskiy
