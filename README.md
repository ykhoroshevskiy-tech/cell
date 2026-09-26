# cell

[![CI](https://github.com/ykhoroshevskiy-tech/cell/actions/workflows/ci.yml/badge.svg)](https://github.com/ykhoroshevskiy-tech/cell/actions/workflows/ci.yml)
[![Coverage](https://codecov.io/gh/ykhoroshevskiy-tech/cell/graph/badge.svg)](https://codecov.io/gh/ykhoroshevskiy-tech/cell)
![Go](https://img.shields.io/badge/go-1.26-00ADD8?logo=go&logoColor=white)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

Sandboxed coding-agent runtime, per-session Firecracker microVMs.

```
sudo cell launch ──▶ firecracker ──▶ microVM 
      │                                  │ repo on /dev/vdb
host opencode TUI ◀── ssh -L tunnel ─────┤ opencode serve :4096
host repo ◀── auto rsync ◀───────────────┤ node/uv/python in guest
```

Tested with [OpenCode](https://github.com/sst/opencode), where the TUI runs on the host and connects over an SSH
tunnel, and [Claude Code](https://github.com/anthropics/claude-code), which runs inside the guest over SSH and tmux.
Each guest gets its own kernel. Requires `/dev/kvm` and a Linux host.

## Install & quick start

```sh
go build -o cell ./cmd/cell && sudo install -m 755 cell /usr/bin/cell
sudo cell bootstrap       # kernel, rootfs, firecracker → /var/lib/cell
sudo cell launch --agent opencode --repo /path/to/repo   # or --agent claude
```


Mutating commands require root. The VM keeps running after the TUI exits until `stop` is called.
After a host reboot: `sudo cell start --session <id>`.

## Commands

| Command | Description |
|---|---|
| `bootstrap` | Download/build kernel, rootfs, firecracker |
| `launch` | Stage repo, boot VM, attach (`--agent opencode\|claude\|none`) |
| `start` / `attach` | Boot an existing session / reconnect the TUI |
| `stop` / `rm` | Stop one session or all of them; `rm` frees disk and IP |
| `ps` / `logs` / `status` / `verify` | List sessions, tail serial log, probe health |
| `ssh` | Debug shell + tmux inside the guest |
| `pull` | Rsync guest workspace back to host repo |
| `rescue` | Extract workspace directly from the project disk |

## Security & threat model

Protected:  
- Guest kernel is separate from the host kernel.
- Guest-to-guest and guest-to-private-network traffic is dropped at the bridge and firewall.

Not protected:

- Guest egress is unrestricted: NAT to the full internet, no per-guest policy.
- Host-side `cell` commands run as root.
- Agent API keys live inside the guest, in plaintext.

## Configuration

| Setting | Default |
|---|---|
| `CELL_DATA_DIR` | `/var/lib/cell` |
| `CELL_PROJECT_DISK_SIZE_MB` | `3072` |
| `CELL_VCPU_COUNT` / `CELL_MEM_SIZE_MIB` | `4` / `8192` |
| `CELL_AUTO_PULL` / `CELL_AUTO_PULL_INTERVAL_SEC` | `true` / `30` |

Host requires `curl`, `tar`, `mkfs.ext4`, `ssh`, `rsync`, `debootstrap`, Go 1.26+ to build, and [OpenCode](https://github.com/sst/opencode) for the tunnelled-TUI path.

## License

[MIT](LICENSE) © Yuri Khoroshevskiy
