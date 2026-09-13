# Ignore system SSH config for guest connections

Date: 2026-08-22

## Problem

`cell -v launch` waits on guest SSH after TCP/22 is already open. `pstree` on the hung `cell` child:

```
cell ─── ssh ─── sss_ssh_knownhosts
```

Guest serial shows `sshd started` and `runtime ready`. Host TCP to `guest:22` is `ESTAB`. The hang is OpenSSH on the **host** running SSSD `KnownHostsCommand` from `/etc/ssh/ssh_config` (or `ssh_config.d`).

`UserKnownHostsFile=/dev/null` does not disable `KnownHostsCommand`. `-o KnownHostsCommand=none` can still lose to a later `Match final all` in distro ssh_config (IPA/SSSD snippets).

`cell pull` builds its own `ssh -e` string and has the same gap.

## Goal

Every host→guest SSH that cell starts ignores `/etc/ssh/ssh_config` and user `~/.ssh/config`, so SSSD/IPA helpers never run.

## Non-goals

- Changing ConnectTimeout, BatchMode, health curl, wait-ready logging.
- Firewall, bridge, e2e IP allocation.
- Replacing OpenSSH with a Go SSH client.

## Design

1. `sshBaseArgs` starts with `-F`, `/dev/null`. OpenSSH then ignores system and user config files. Keep the existing `-i` / `IdentitiesOnly` / `BatchMode` / `ConnectTimeout=5` / `StrictHostKeyChecking=no` / `UserKnownHostsFile=/dev/null`.
2. Do **not** rely on `-o KnownHostsCommand=none` alone.
3. `cell pull` rsync `-e` must use the same flags. Export `RemoteShell(keyPath string) string` from `internal/ssh`: `"ssh "` + `sshBaseArgs` joined by spaces. `pull.go` uses that instead of a parallel format string.

## Files

- `internal/ssh/attach.go` — `sshBaseArgs`, `RemoteShell`
- `internal/ssh/attach_test.go` — `TestTunnelSSHArgs` includes `-F` `/dev/null`; add `TestRemoteShell`
- `internal/sync/pull.go` — rsync `-e` from `ssh.RemoteShell`

## Success

- `go test ./internal/ssh ./internal/sync` passes.
- `TestTunnelSSHArgs` / `TestRemoteShell` require `-F` `/dev/null`.
- After rebuild, a wait-ready SSH probe on this host does not spawn `sss_ssh_knownhosts`.
