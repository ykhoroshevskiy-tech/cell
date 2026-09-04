# Absolute repo_source

Date: 2026-09-04

## Problem

`cell launch --repo ./` stores `repo_source` as `./` in `session.json`. Later `cell pull` and auto-pull pass that string to rsync, which resolves it against the **current cwd** (often `$HOME` or `/root` under sudo), not the directory where launch ran.

## Goal

Every new session persists `repo_source` as an absolute, cleaned directory path. Pull to the session default dest (or `--dest`) fails if that path is not absolute — it must not silently write to cwd.

## Non-goals

- Auto-migrating existing `session.json` with relative `repo_source`.
- `filepath.EvalSymlinks`.
- `cell doctor` / rewrite CLI.
- Cleaning files already pulled into `$HOME`.
- Changing `cell rescue`.

## Design

1. `ResolveRepoSource(path string) (string, error)` in `internal/session`:
   - empty path → error
   - `filepath.Abs` then `filepath.Clean`
   - `os.Stat`: path must exist and be a directory
2. `prepareSession` stores the resolved path in `RepoSource` before first `saveSession`.
3. After dest is chosen (`opts.Dest` or `session.RepoSource`), `ValidateDest` rejects non-absolute paths with an error that includes the dest string. Do **not** call `Abs` on pull (cwd is the bug). Unsafe system roots stay rejected as today.
4. `loadSession` does not rewrite relative `repo_source`. Old sessions stay broken until JSON is patched by hand.

## Files

- `internal/session/repopath.go` — `ResolveRepoSource`
- `internal/session/repopath_test.go`
- `internal/session/lifecycle.go` — `prepareSession` uses `ResolveRepoSource`
- `internal/sync/pull.go` — `ValidateDest` requires absolute path
- `internal/sync/pull_test.go` — relative dest rejected
- `README.md` — one line that `--repo` is stored absolute

## Success

- `go test ./internal/session ./internal/sync` passes.
- `cell launch --repo ./` writes an absolute directory into `session.json`.
- `cell pull` with `repo_source` `./` errors instead of rsyncing into cwd.
