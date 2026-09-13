# Architecture

How `cell` works under the hood. User-facing reference lives in
[README.md](README.md); every tunable is listed in
[CONFIGURATION.md](CONFIGURATION.md).

## Lifecycle

```
prepare            boot                serve                attach             sync
repo → copy     →  mkfs project   →  firecracker VM  →   opencode serve  →  ssh -L tunnel
   staged tree      disk image         kernel+rootfs+disk    (in-guest)         host TUI
```

1. **Prepare session** — a random 12-hex session id; per-session directory under
   `<data>/session-data/<id>/` holds `session.json`, `network.json`,
   `vm-config.json`, SSH keypair (`id_ed25519`, 0600), `serial.log`,
   `project.ext4` and `firecracker.socket`.
2. **Stage** — the repo is copied (rsync, excludes `__pycache__`, `node_modules`,
   `.venv`, `images`, `session-data`) into a temp root together with
   `.filter/` (`authorized_keys`, `opencode-server.pass`), then written into a
   fresh ext4 image via `mkfs.ext4 -d` — that image is the project disk.
3. **Boot** — Firecracker starts with the pinned kernel, read-only rootfs
   (Ubuntu 24.04 debootstrap), and the project disk as `/dev/vdb`. The guest
   entry script mounts `/project` and `/usr/local` from the project disk and
   starts the agent command in tmux.
4. **Serve** — the agent runs `opencode serve --hostname 127.0.0.1
   --port 4096` inside the guest. The in-guest server is protected by basic
   auth; the random password lives in the session dir and on the disk root
   (`.filter/opencode-server.pass`).
5. **Wait ready** — serial log is streamed and scanned for fatal patterns
   (kernel panic, guest-init errors); readiness = VM alive + SSH port open +
   server health probe (`curl` in the guest over loopback with the password).
6. **Attach** — the host picks a free local port, starts `ssh -N -L
   <port>:127.0.0.1:4096` to the guest, then runs `opencode attach
   http://127.0.0.1:<port> --dir /project --continue -p <password>` in the
   foreground. The TUI runs on the host; the agent runs in the VM.
7. **Sync** — auto-pull (default every 30s while attached) rsyncs the guest
   workspace back to the host repo path over SSH (`-a --no-owner --no-group
   --chown=<host owner>`, `--delete` only with explicit `--delete`);
   `.filter` is excluded both ways.

## Networking

All sessions share one Linux bridge, `cell0` (`172.16.107.1/24`). Guest IPs
come from a stable lease pool (`.2`–`.254`) persisted in `session.json`
(`network_version: 2`).

- Per session a TAP device (`ctap-<id>`) is created and enslaved to `cell0`
  with **no host-side IP**; bridge **port isolation** blocks guest-to-guest L2.
- iptables chains (`CELL_INPUT`, `CELL_FORWARD`, `CELL_NAT`) are created
  idempotently: established/related accepted, host `:8080` accepted,
  guest-sourced input dropped, guest→private CIDR and guest→guest forwarding
  dropped, subnet egress accepted + one MASQUERADE rule, `ip_forward=1`.
- Before `launch`, `start`, `stop`, and `rm` the runtime converges
  bridge/TAP/firewall state to the live session set under a global lock
  (`/run/lock/cell-network.lock`, `flock`): stale TAPs are removed, live
  sessions re-attached, dead processes repaired in `session.json`.
- The firewall runs on the host; guests get unrestricted outbound internet
  access (no per-guest egress policy). See the security section in
  [README.md](README.md#security--threat-model).

## Privilege model

Read-only commands (`ps`, `logs`, `version`, `help`, `completion`,
`__complete`) only read world-readable state and run without root. Every
mutating command requires root and exits with
`cell: <cmd> requires root — run: sudo cell <cmd>` otherwise (central gate in
`internal/cli/root.go`).

VM liveness is computed from `/proc/<pid>/cmdline` matched against the session
API socket — world-readable, so `ps` stays truthful for root-owned VMs without
root.

## Bootstrap artifacts

`sudo cell bootstrap` pins and caches into `<data>/images/`:

- `vmlinux-<kernel pin>` (symlink `vmlinux`) — Firecracker CI kernel
- `bin/firecracker-<version>` (symlink `bin/firecracker`) + `jailer` from the
  same release
- `rootfs.ext4` — built via `debootstrap noble --variant=minbase`, guest
  packages (`openssh-server tmux zsh rsync curl git sudo ca-certificates`),
  Node LTS + uv + CPython into `/usr/local` (tarballs pinned by version),
  sshd config, `agent` user with passwordless sudo, guest-init scripts
  (embedded, written into the rootfs at build time)
- optional Superpowers install (opt-in, see
  [CONFIGURATION.md](CONFIGURATION.md))

A stamp file (`rootfs.ext4.squashfs-version`) records the build inputs; any
change rebuilds the rootfs.

## Host reboot recovery

Session state is persistent (`/var/lib/cell`). After a host reboot,
`sudo cell start --session <id>` boots the existing project disk again; the
network lease and TAP are reconciled before boot.
