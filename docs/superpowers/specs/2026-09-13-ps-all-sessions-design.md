# Design: `cell ps` lists all sessions; shell completion command

## Problem

`cell ps` showed running sessions only by default and required `--all` for the
complete listing — the opposite of what `ps` means everywhere else. Tabbing
sessions existed nowhere; the only way to enumerate sessions was reading
`session.json` by hand.

## Goal

`cell ps` lists every session (running and stopped) by default; a `completion`
command generates shell completion scripts for zsh/bash/fish/powershell.

## Non-Goals

- Filtering flags on `ps` (`--running` etc. — add when needed).
- Aliases inside `cell` itself (the oh-my-zsh plugin covers that).

## Design

- `internal/cli/ps.go`: drop the `--all` flag; `sm.List(ctx, false)` always.
  `--json` unchanged (machine-readable, all sessions).
- `internal/cli/root.go`: explicit `completion [bash|zsh|fish|powershell]`
  command delegating to cobra generators (zsh: `GenZshCompletion`,
  bash: `GenBashCompletionV2(desc=true)`, fish, powershell-with-descriptions).
  In the read-only allowlist — runs without root.
- Cobra's default `completion`/`help`/`__complete` stay allowlisted.

## Gates

1. `./cell ps` → exit 0, prints the header plus all sessions.
2. `./cell ps --all` → `error: unknown flag: --all`.
3. `./cell completion zsh | head -1` → `#compdef cell`.
4. `go build ./... && go vet ./... && go test ./...` → green.

## Gate evidence

```
$ ./cell ps; echo rc=$?
rc=0
$ ./cell ps --all
error: unknown flag: --all
$ ./cell completion zsh | head -1
#compdef cell
$ go build ./... && go vet ./... && go test ./...   # green
```
