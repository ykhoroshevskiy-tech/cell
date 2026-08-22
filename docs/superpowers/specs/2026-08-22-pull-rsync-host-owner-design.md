# Pull rsync ownership from host dest dir

Date: 2026-08-22

## Problem

`cell pull` (and auto-pull) runs `rsync -av` as root. `-a` preserves guest uid/gid. Guest `agent` is typically uid 1001, so files on the host land as uid 1001 instead of the person who owns the repo.

Dropping `-o` without remapping would make them `root:root`.

## Goal

After rsync, files under the pull destination are owned by the **uid/gid of that destination directory as it existed before rsync**. Same for auto-pull (`PullWorkspace`).

## Non-goals

- `cell rescue` (separate copy path).
- Configurable owner (`CELL_PULL_OWNER`, `SUDO_UID`).
- Changing guest `chown_repo` / agent uid inside the VM.
- Preserving guest-only uids on the host.

## Design

1. Helper `destOwner(path string) (uid, gid int, err error)`:
   - `Lstat(path)` if it exists.
   - Else `Lstat(filepath.Dir(path))`.
   - Read `uid`/`gid` from `syscall.Stat_t` on Linux.
2. `PullWorkspace` adds to rsync args (after `-av`):
   `--no-owner --no-group --chown=<uid>:<gid>`
   Keep `-a` (permissions, times, `-D`). `--chown` requires root, which `cell` already is.
3. If `destOwner` fails, pull fails (do not silently write as root or 1001).

## Files

- `internal/sync/pull.go` — exported `DestOwner(path string) (uid, gid int, err error)`, rsync flags
- `internal/sync/pull_test.go` — `TestDestOwner` on a temp dir (uid/gid match the dir)

## Success

- `go test ./internal/sync` passes.
- After `sudo cell pull`, new/updated files under the host repo have the same uid/gid as the repo directory had before the pull.
