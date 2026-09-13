# Design: Run cell runtime without sudo

## Problem

`cell` currently requires `sudo` for every runtime command because it needs `/dev/kvm`, Linux bridge/TAP/iptables, `mkfs.ext4 -d`, `/var/lib/cell`, and `/run/lock/cell-network.lock`. The user wants Docker-style UX: runtime without sudo, while bootstrap can stay privileged.

## Goal

A user in the `cell` group can run `cell launch`, `stop`, `attach`, `ps`, `pull`, `start`, `rm`, `logs`, `status`, `verify`, `rescue`, `ssh` without `sudo`. `cell bootstrap` still requires root.

## Non-Goals

- Rootless bootstrap (debootstrap, rootfs mkfs, `/usr/bin` install).
- User-mode networking (slirp4netns) — we keep the existing bridge/TAP/iptables model.
- Per-user data directories.
- Removing the `kvm` group requirement for `/dev/kvm`.

## Design

### Privilege model

`cell` is installed with file capabilities; the user is added to the `cell` group:

```sh# Design: Run cell runtime without sudo

## Problem

`cell` currently requires `sudo` for every runtime command because it needs `/dev/kvm`, Linux bridge/TAP/iptables, `mkfs.ext4 -d`, `/var/lib/cell`, and `/run/lock/cell-network.lock`. The user wants Docker-style UX: runtime without sudo, while bootstrap can stay privileged.

## Goal

A user in the `cell` group can run `cell launch`, `stop`, `attach`, `ps`, `pull`, `start`, `rm`, `logs`, `status`, `verify`, `rescue`, `ssh` without `sudo`. `cell bootstrap` still requires root.

## Non-Goals

- Rootless bootstrap (debootstrap, rootfs mkfs, `/usr/bin` install).
- User-mode networking (slirp4netns) — we keep the existing bridge/TAP/iptables model.
- Per-user data directories.
- Removing the `kvm` group requirement for `/dev/kvm`.

## Design

### Privilege model

`cell` is installed with file capabilities; the user is added to the `cell` group:

```sh
sudo groupadd -f cell
sudo usermod -aG cell $USER
sudo setcap 'cap_net_admin,cap_net_raw,cap_dac_override+ep' /usr/bin/cell
```

- `cap_net_admin` — bridge, TAP, iptables, sysctl writes.
- `cap_net_raw` — raw sockets (Firecracker may need them).
- `cap_dac_override` — read/write root-owned files under `/var/lib/cell` and inside the project disk image (`mkfs.ext4 -d` writes root-owned files).

`/dev/kvm` access continues via the existing `kvm` group.

### Directory layout

`/var/lib/cell` becomes group-writable with setgid:

```
/var/lib/cell               root:cell  2775
/var/lib/cell/session-data  root:cell  2775
/var/lib/cell/artifacts     root:cell  2775
```

The setgid bit (`2775`) makes new files and directories inherit the `cell` group. `bootstrap` (run with sudo) creates or fixes this layout once. Runtime commands run without sudo and create session subdirectories inside.

The network lock stays at `/run/lock/cell-network.lock` but is created by bootstrap with owner `root:cell` and mode `0664`.

### Root vs capability

| Operation | Privilege mechanism | Reason |
|-----------|---------------------|--------|
| `bootstrap` | `sudo` (root) | debootstrap, chroot apt, rootfs mkfs, install to `/usr/bin`, setcap, chown `/var/lib/cell` |
| `cell net-setup` (hidden) | `cap_net_admin` + `cap_dac_override` | bridge, TAP, iptables, sysctl, lock file |
| Runtime commands | capabilities + `cell` group | project disk mkfs, firecracker, ssh, rsync |
| `mkfs.ext4 -d` for project disk | `cap_dac_override` | image contents are root-owned; image file lives in group dir |
| `/dev/kvm` | `kvm` group | already works |

`net-setup` is a hidden subcommand: `cell` invokes it automatically before network operations when the bridge or TAP is missing. It is not a separate sudo helper — it is the same binary running with capabilities.

### Code changes

- `cmd/cell/main.go`: remove the hard `os.Geteuid() != 0` exit. Instead, check that the user is in the `cell` group and that the binary has the required capabilities. If not, print: `cell runtime requires group 'cell' — run: sudo cell bootstrap` (or `sudo cell doctor` if we add it later).
- `internal/cli/root.go` (or `main.go`): `bootstrap` explicitly requires root (`os.Geteuid() == 0`).
- `internal/network/*`: `ip` / `iptables` calls stay; they now succeed under `cap_net_admin`. Lock path unchanged; bootstrap fixes permissions.
- `internal/session/lifecycle.go`: `mkfs.ext4 -d` stays; `cap_dac_override` allows writing root-owned files into the image. The image itself is created in `/var/lib/cell/session-data/<id>/project.ext4` with group `cell`.
- `internal/config/cell_config.go`: `CELL_DATA_DIR` default remains `/var/lib/cell`.
- No changes to `guestinit/` or pull logic.

### Security boundaries

- `cap_dac_override` lets `cell` bypass Unix file permissions. This is a deliberate trade-off, same as the Docker daemon. Mitigation: the binary is built from this repo and installed via `sudo install`.
- `cap_net_admin` lets `cell` modify iptables and bridges. Already required for functionality.
- Membership in the `cell` group grants VM launch rights. Adding a user to the group remains a root operation (done by bootstrap or the admin).
- `net-setup` does not take user-controlled paths from the environment; lock and bridge names are compiled in.

### Error handling

| Case | Behavior |
|------|----------|
| User not in `cell` group | `cell: runtime requires group 'cell' — run: sudo cell bootstrap` |
| Binary missing capabilities | `cell: missing capabilities — reinstall with sudo cell bootstrap` |
| `cell bootstrap` as non-root | `cell: bootstrap requires root` |
| `/var/lib/cell` wrong owner/perms | `bootstrap` fixes it; runtime errors point to bootstrap |
| `mkfs.ext4` fails with EPERM | clear message to re-run bootstrap |

### Testing

- **Unit (Go):** `main` does not exit when euid != 0 but group `cell` and capabilities are present.
- **Unit (Go):** `main` exits with the group message when the user is not in `cell`.
- **Unit (Go):** `network.lock` works with the new permissions model.
- **Manual:** `sudo cell bootstrap` → `newgrp cell` → `cell launch` succeeds without sudo.
- **Manual:** `cell ps`, `cell attach`, `cell pull`, `cell stop` succeed without sudo.
- **Regression:** `sudo cell bootstrap` still works exactly as before.

No new KVM e2e tests.

## Success criteria

1. After `sudo cell bootstrap` and group membership, `cell launch` runs without sudo.
2. All existing runtime commands work without sudo.
3. `cell bootstrap` still requires root and produces a working rootfs.
4. Existing tests pass; new unit tests cover the privilege checks.

sudo groupadd -f cell
sudo usermod -aG cell $USER
sudo setcap 'cap_net_admin,cap_net_raw,cap_dac_override+ep' /usr/bin/cell
```

- `cap_net_admin` — bridge, TAP, iptables, sysctl writes.
- `cap_net_raw` — raw sockets (Firecracker may need them).
- `cap_dac_override` — read/write root-owned files under `/var/lib/cell` and inside the project disk image (`mkfs.ext4 -d` writes root-owned files).

`/dev/kvm` access continues via the existing `kvm` group.

### Directory layout

`/var/lib/cell` becomes group-writable with setgid:

```
/var/lib/cell               root:cell  2775
/var/lib/cell/session-data  root:cell  2775
/var/lib/cell/artifacts     root:cell  2775
```

The setgid bit (`2775`) makes new files and directories inherit the `cell` group. `bootstrap` (run with sudo) creates or fixes this layout once. Runtime commands run without sudo and create session subdirectories inside.

The network lock stays at `/run/lock/cell-network.lock` but is created by bootstrap with owner `root:cell` and mode `0664`.

### Root vs capability

| Operation | Privilege mechanism | Reason |
|-----------|---------------------|--------|
| `bootstrap` | `sudo` (root) | debootstrap, chroot apt, rootfs mkfs, install to `/usr/bin`, setcap, chown `/var/lib/cell` |
| `cell net-setup` (hidden) | `cap_net_admin` + `cap_dac_override` | bridge, TAP, iptables, sysctl, lock file |
| Runtime commands | capabilities + `cell` group | project disk mkfs, firecracker, ssh, rsync |
| `mkfs.ext4 -d` for project disk | `cap_dac_override` | image contents are root-owned; image file lives in group dir |
| `/dev/kvm` | `kvm` group | already works |

`net-setup` is a hidden subcommand: `cell` invokes it automatically before network operations when the bridge or TAP is missing. It is not a separate sudo helper — it is the same binary running with capabilities.

### Code changes

- `cmd/cell/main.go`: remove the hard `os.Geteuid() != 0` exit. Instead, check that the user is in the `cell` group and that the binary has the required capabilities. If not, print: `cell runtime requires group 'cell' — run: sudo cell bootstrap` (or `sudo cell doctor` if we add it later).
- `internal/cli/root.go` (or `main.go`): `bootstrap` explicitly requires root (`os.Geteuid() == 0`).
- `internal/network/*`: `ip` / `iptables` calls stay; they now succeed under `cap_net_admin`. Lock path unchanged; bootstrap fixes permissions.
- `internal/session/lifecycle.go`: `mkfs.ext4 -d` stays; `cap_dac_override` allows writing root-owned files into the image. The image itself is created in `/var/lib/cell/session-data/<id>/project.ext4` with group `cell`.
- `internal/config/cell_config.go`: `CELL_DATA_DIR` default remains `/var/lib/cell`.
- No changes to `guestinit/` or pull logic.

### Security boundaries

- `cap_dac_override` lets `cell` bypass Unix file permissions. This is a deliberate trade-off, same as the Docker daemon. Mitigation: the binary is built from this repo and installed via `sudo install`.
- `cap_net_admin` lets `cell` modify iptables and bridges. Already required for functionality.
- Membership in the `cell` group grants VM launch rights. Adding a user to the group remains a root operation (done by bootstrap or the admin).
- `net-setup` does not take user-controlled paths from the environment; lock and bridge names are compiled in.

### Error handling

| Case | Behavior |
|------|----------|
| User not in `cell` group | `cell: runtime requires group 'cell' — run: sudo cell bootstrap` |
| Binary missing capabilities | `cell: missing capabilities — reinstall with sudo cell bootstrap` |
| `cell bootstrap` as non-root | `cell: bootstrap requires root` |
| `/var/lib/cell` wrong owner/perms | `bootstrap` fixes it; runtime errors point to bootstrap |
| `mkfs.ext4` fails with EPERM | clear message to re-run bootstrap |

### Testing

- **Unit (Go):** `main` does not exit when euid != 0 but group `cell` and capabilities are present.
- **Unit (Go):** `main` exits with the group message when the user is not in `cell`.
- **Unit (Go):** `network.lock` works with the new permissions model.
- **Manual:** `sudo cell bootstrap` → `newgrp cell` → `cell launch` succeeds without sudo.
- **Manual:** `cell ps`, `cell attach`, `cell pull`, `cell stop` succeed without sudo.
- **Regression:** `sudo cell bootstrap` still works exactly as before.

No new KVM e2e tests.

## Success criteria

1. After `sudo cell bootstrap` and group membership, `cell launch` runs without sudo.
2. All existing runtime commands work without sudo.
3. `cell bootstrap` still requires root and produces a working rootfs.
4. Existing tests pass; new unit tests cover the privilege checks.

## Addendum 2026-09-06: runtime still failed without sudo — gaps found and fixed

The original design shipped (commit 74f9001) but `cell stop` and `cell launch`
still failed without sudo while `cell ps` worked. Root causes:

1. **Lock chmod EPERM.** `PrepareNetworkLock` chmod'd the bootstrap-owned
   `root:cell 0664` lock on every runtime call. chmod requires ownership or
   `CAP_FOWNER`; `cap_dac_override` does not grant it. Fixed: chmod only when
   the caller owns the lock (or is root); lock is now created 0664.
2. **Ambient raise raced with Go threading.** `PR_CAP_AMBIENT_RAISE` requires
   the cap in permitted AND inheritable; exec preserves the (empty)
   inheritable set, so the raise silently no-oped. And even after copying
   permitted→inheritable via capset, the raise is thread-local: goroutines
   migrate between OS threads, so children inherited caps unreliably
   (observed: identical children alternating CapEff 0x0 / 0x3000).
   Fixed two ways: capset before the ambient raise in `RaiseAmbientNetCaps`,
   and `SysProcAttr{AmbientCaps: …}` (Go ≥1.19 raises the caps inside the
   fork child before execve) applied to network children and firecracker,
   gated on the process actually holding the caps.
3. **kvm group.** Under sudo, firecracker opened `/dev/kvm` as root. Running
   as the invoking user requires the `kvm` group. Bootstrap now adds
   `$SUDO_USER` to `kvm` too, and `startVM` fails fast with an actionable
   message.
4. **Legacy sudo-created sessions.** Root-owned `session.json` broke
   `saveSession` (EACCES) and root-owned firecracker processes could not be
   signalled. Bootstrap now runs a one-time `chgrp -R cell` +
   `chmod -R g+rwX` over session-data (SSH private keys re-tightened to
   0600), and `hypervisor.Stop` reports EPERM with a `sudo cell stop` hint
   instead of silently pretending success.
5. **Bootstrap/rescue UX.** Both auto-elevate: `privilege.MaybeElevate`
   re-execs `sudo env CELL_*=… cell <cmd>` (CELL_* forwarded because sudo
   resets the environment); exit code propagates. `rescue` is exempt from
   the cell-group check in `NeedsRuntimeAccess`.

Correction to the earlier note: per capabilities(7) ("Capabilities and
execution of programs by root"), when a uid-0 process execs a file, the file
inheritable/permitted sets are ignored and treated as all-ones — file caps do
NOT strip root's full set, so in-process root operations (chown) were never
EPERM-limited under `sudo cell bootstrap`. `ChownRootCell` still shells out to
`chown` as root for robustness.

Verified in the cell guest (no KVM, but identical capability semantics):
`/tmp/captest/cell net-setup` as a plain user with file caps
`cap_net_admin,cap_net_raw+eip cap_dac_override+ep` completed the full
lock/bridge/firewall sequence with every `ip`/`iptables` child carrying
`CapEff=0x3000` across repeated runs; `sudo cell net-setup` and
`cell ps` regression-passed; `cell bootstrap` auto-elevated via sudo and
failed only at the (intentionally invalid) artifact prefix, exit code
propagated.
