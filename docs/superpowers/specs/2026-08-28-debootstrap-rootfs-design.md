# Debootstrap guest rootfs + Node LTS + Superpowers

Date: 2026-08-28

## Problem

Firecracker CI squashfs has `apt` binaries but an empty dpkg database, so the guest cannot `apt install`. Node/npm are missing, so OpenCode cannot install the Superpowers plugin listed in `opencode.json`. The project disk (`CELL_PROJECT_DISK_SIZE_MB=1024`) fills quickly with agent-home caches. `guest-entry.sh` overwrites `opencode.json` every boot.

## Goal

Build guest rootfs with `debootstrap` Ubuntu 24.04 (noble) so apt works. Pin Node LTS into `/usr/local`. Install Superpowers into `/opt/opencode-plugins` at bootstrap. Default project disk 3 GiB. Do not grow rootfs with the project disk. Do not overwrite an existing `opencode.json`.

## Non-goals

- systemd as PID 1 (keep `init=/opt/guest-init/guest-entry.sh`)
- Node Current (v26); default is LTS v24.20.0, override via `CELL_NODE_VERSION`
- Baking operator LLM provider URLs into cell
- Growing existing session disks (new `launch` only)

## Design

- Host: `debootstrap --variant=minbase noble`, then chroot `apt-get install` openssh-server tmux zsh rsync curl git sudo ca-certificates. `ssh-keygen -A`. `DEBIAN_FRONTEND=noninteractive`.
- Stop downloading squashfs. Stamp: `debootstrap:noble+node:{NodeVersion}`.
- Node: official tarball `https://nodejs.org/dist/{ver}/node-{ver}-{linux-x64|linux-arm64}.tar.xz` → `/usr/local`.
- Superpowers: `npm install --prefix /opt/opencode-plugins superpowers@git+https://github.com/obra/superpowers.git`
- Keep passwordless sudoers for `agent`.
- `CELL_PROJECT_DISK_SIZE_MB` default 3072. `CELL_ROOTFS_SIZE_MB` default 4096 (independent).
- `guest-entry.sh`: write default `opencode.json` only if missing; plugin path `/opt/opencode-plugins/node_modules/superpowers`.

## Success

- `go test ./...` passes.
- After `sudo cell bootstrap --rebuild-rootfs`: guest `apt-get` has a dpkg DB, `node -v` is v24.20.0, Superpowers exists under `/opt/opencode-plugins`, `sudo -n true` as agent.
- New launch project disk is 3072 MiB. Existing `opencode.json` on agent-home is left intact.
