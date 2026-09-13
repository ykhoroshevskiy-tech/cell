# Writable `/usr/local` on a read-only rootfs

Date: 2026-08-29

## Problem

Guest rootfs is Firecracker read-only (`IsReadOnly: true`). `/usr/local/bin` lives on that rootfs, so `npm i -g`, copying tools, etc. fail with EROFS even as root/sudo.

## Goal

Make `/usr/local` writable for the session without making the whole rootfs RW.

## Non-goals

- Writable rootfs
- Overlayfs (kernel extra); copy+bind is enough
- Refreshing a seeded copy when Node in rootfs changes (delete `.filter/usr-local` and reboot)

## Design

On boot, seed `/project/.filter/usr-local` from the RO `/usr/local` once, bind-mount it over `/usr/local`, `chown` to `agent`. Same pattern as agent-home.

## Success

- `guest-entry.sh` bind-mounts a project-disk copy over `/usr/local`.
- First seed copies `node`/`npm` so PATH still works.
- `go test ./guestinit/` passes.
