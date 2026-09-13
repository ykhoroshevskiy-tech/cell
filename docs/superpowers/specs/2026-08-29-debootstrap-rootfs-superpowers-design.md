# Debootstrap guest rootfs and install Superpowers for real

Date: 2026-08-29

## Problem

HEAD still builds rootfs from Firecracker CI squashfs + extracted jammy debs. Git and friends miss shared libraries. Guest `apt` is not a real Ubuntu install. Superpowers was only a git URL in `opencode.json`; OpenCode never got a tree on disk. Old agent-home config is kept if the file already exists (`if [ ! -f CFG ]`), so a git spec or empty plugin list sticks forever.

Working tree already started debootstrap noble + `npm install` into `/opt/opencode-plugins`. This spec is that work, plus the two holes: guest apt sources, and **always** overwrite OpenCode config to the local plugin path.

## Goal

- `cell bootstrap --rebuild-rootfs` produces Ubuntu 24.04 (noble) via `debootstrap --variant=minbase`, then `apt-get install` in chroot (ssh, tmux, zsh, rsync, curl, git, sudo, ca-certificates).
- In-guest `apt-get` works (sources.list with main+universe, updates, security; NAT already exists).
- Superpowers is on the rootfs at `/opt/opencode-plugins/node_modules/superpowers` (npm at bootstrap). Config plugin path is that directory, not a git URL.
- Default `opencode.json` is written only if missing: `permission: allow` + local plugin path. No 8080/kimi provider — OpenCode Go subscription. Existing files are not overwritten.

## Non-goals

- Bind-mounting the host apt cache or using the host’s Ubuntu release.
- Keeping squashfs / jammy `dpkg-deb -x` as a fallback.
- Downloading the plugin at guest runtime.

## Design

1. Drop `unsquashfs` + `ensureJammyDebs` from `buildRootfs`. Host needs `debootstrap` on PATH.
2. `debootstrap --variant=minbase noble <root> http://archive.ubuntu.com/ubuntu`.
3. Write `/etc/apt/sources.list` (noble main universe, updates, security) then chroot `apt-get update && apt-get install -y --no-install-recommends` the guest package list.
4. Keep host-side Node into `/usr/local`. Chroot `npm install --prefix /opt/opencode-plugins superpowers@git+https://github.com/obra/superpowers.git`. Fail bootstrap if that directory is missing.
5. `guest-entry.sh` writes agent-home `opencode.json` **only if missing**:

```json
{
  "$schema": "https://opencode.ai/config.json",
  "permission": "allow",
  "plugin": ["/opt/opencode-plugins/node_modules/superpowers"]
}
```

No custom provider or 8080. Existing configs (including OpenCode Go subscription) stay.

6. Rebuild stamp includes debootstrap+node so old squashfs rootfs is not reused. Operator: `sudo ./cell bootstrap --rebuild-rootfs` then stop/start (or new launch).

## Files

- `internal/bootstrap/bootstrap.go` (and tests that still mention jammy/squashfs extract)
- `guestinit/guest-entry.sh`
- `README.md` host tool `debootstrap`; squashfs pin unused for rootfs

## Success

- `ldd $(which git)` in a new guest has no `not found`.
- `test -d /opt/opencode-plugins/node_modules/superpowers`.
- Guest `opencode.json` plugin is the local path; `apt-get update` in the guest succeeds.
