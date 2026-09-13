# Guest uv + pinned CPython in rootfs

Date: 2026-08-29

## Problem

The guest has Node in `/usr/local` but no Python toolchain. Agents need `uv` and a current CPython without apt's 3.12. A `launch --snapshot` / `--language` flag is deferred.

## Goal

Install pinned `uv` and a pinned CPython into the guest rootfs at bootstrap, on PATH as `uv` and `python3`, same place as Node (`/usr/local`). Existing sessions pick it up after `bootstrap --rebuild-rootfs` plus a new seed of `.filter/usr-local` (or a new launch).

## Non-goals

- `cell launch --snapshot` / `--language` / `--env`
- Re-pinning or reinstalling Node
- apt `python3` as the agent interpreter
- Writable rootfs
- Per-session Python (install is in the RO rootfs; the `/usr/local` bind copy seeds it)

## Design

Same pattern as Node: host downloads a pinned tarball into `ImagesDir`, extracts into the chroot `/usr/local`.

- `CELL_UV_VERSION` default `0.12.7`. Tarball: GitHub `astral-sh/uv` release `uv-{arch}-unknown-linux-gnu.tar.gz` (`x86_64` / `aarch64`). Extract `uv` to `/usr/local/bin`.
- `CELL_PYTHON_VERSION` default `3.13` (minor pin; `uv python install` picks the current 3.13 patch at bootstrap). After `uv` is on PATH in the chroot: `UV_PYTHON_INSTALL_DIR=/usr/local/share/uv/python` and `UV_PYTHON_BIN_DIR=/usr/local/bin`, then `uv python install --default {CELL_PYTHON_VERSION}`. That puts `python` / `python3` on PATH without apt.
- Cache tarballs next to the Node cache under `ImagesDir`.
- Rootfs stamp appends `+uv:{UvVersion}+py:{PythonVersion}` so a pin bump rebuilds.
- `guestCustomizeScript` adds `command -v uv` and `command -v python3`.
- Defaults live in `config.Default()` like `NodeVersion`. Empty env falls back to those defaults.

## Files

- `internal/config/cell_config.go` (+ tests)
- `internal/bootstrap/bootstrap.go` (+ tests: URL, stamp, customize)
- `README.md` (env table + one line)

## Success

- `go test ./...` passes.
- After `sudo cell bootstrap --rebuild-rootfs`, guest `uv --version` is 0.12.7 and `python3 --version` is 3.13.x.
- Stamp changes if either pin changes.
