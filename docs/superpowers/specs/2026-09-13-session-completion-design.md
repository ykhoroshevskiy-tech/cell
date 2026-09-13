# Design: dynamic `--session` flag completion

## Problem

`--session` flags were opaque: no way to discover session IDs from the shell;
users copied IDs from `cell ps` by hand. `sudo cell attach --session <TAB>`
offered nothing.

## Goal

Every `--session` flag completes with live sessions as
`id\t<repo path> (<running|stopped>)` in milliseconds, without root.

## Non-Goals

- Positional-argument completion (IDs are passed via `--session` everywhere).
- Probing SSH or the opencode server during completion.
- Completing `--all`-style pseudo-values.

## Design

- `internal/session/lifecycle.go`: `SessionSummary` +
  `(sm *SessionManager) ListSummaries()` — ReadDir + `session.json` unmarshal
  + `VMRunningForSession` (`/proc/<pid>/cmdline` read only). No `SessionStatus`
  probes (no TCP dials, no SSH).
- `internal/cli/completion.go`: `completeSessionID(cfg)` returns cobra flag
  completions `fmt.Sprintf("%s\t%s (%s)", id, repo, state)` with
  `ShellCompDirectiveNoFileComp`; `registerSessionCompletionAll(root, cfg)`
  wires it onto every subcommand that has a `--session` flag: attach, start,
  stop, rm, ssh, status, verify, logs, pull, rescue.
- Works under `sudo` because completion runs `cell __complete …` as the
  invoking (unprivileged) user; `__complete` is in the read-only allowlist and
  session files are world-readable.
- Empty/broken `session.json` entries are skipped, not fatal.

## Gates

1. With a seeded `CELL_SESSION_DATA_DIR`, `./cell __complete attach --session ''`
   prints `aaa111\t/home/u/my-repo (running)` and
   `:4` (ShellCompDirectiveNoFileComp).
2. Broken session.json entries do not abort completion (skipped).
3. `go build ./... && go vet ./... && go test ./...` → green.

## Gate evidence

```
$ CELL_SESSION_DATA_DIR=/tmp/seed ./cell __complete attach --session ''
aaa111	/home/u/my-repo (running)
:4
Completion ended with directive: ShellCompDirectiveNoFileComp
$ go test ./internal/cli/ -run CompleteSession -v
--- PASS: TestCompleteSessionIDListsSessionsWithRepoPaths
--- PASS: TestListSummariesSkipsBrokenSessions
$ go build ./... && go vet ./... && go test ./...   # green
```
