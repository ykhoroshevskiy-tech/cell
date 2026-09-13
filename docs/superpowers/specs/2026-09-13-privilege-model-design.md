# Design: privilege model — read-only without root, explicit root gate

## Problem

The rootless-runtime attempt (file capabilities + `cell` group + ambient
capabilities + auto-elevation) accumulated walls: lock-file chmod EPERM,
thread-local ambient caps racing goroutine scheduling, `kvm` group requirements,
root-owned session files, stale-credential confusion. Every fix revealed the
next wall; the model was unpredictable ("too difficult").

## Goal

One rule, no exceptions: read-only commands run without root, every mutating
command requires root and fails with an explicit instruction to use sudo.

## Non-Goals

- File capabilities, `cell` group membership, ambient/inheritable caps.
- Auto-elevation (`sudo` re-exec).
- Per-user or group-writable data directories.
- Rootless bootstrap.

## Design

- Gate lives in `internal/cli/root.go` `PersistentPreRunE`:
  `readOnlyCommands = {ps, logs, version, help, completion, __complete,
  __completeNoDesc}` run for any user; everything else requires
  `os.Geteuid() == 0`, otherwise:
  `cell: <cmd> requires root — run: sudo cell <cmd>` (exit 1).
- Deleted: `internal/privilege` (ambient caps, elevation, group gate),
  `bootstrap.PrepareRuntimeEnvironment`, `network.ChownRootCell`,
  `PrepareNetworkLockForGroup`, kvm-group add, legacy chgrp migration,
  lock chmod guard, `ensureAuthorizedKeys` rootless tolerance.
- Kept: `hypervisor.Stop` EPERM honesty (fails with a sudo hint instead of
  reporting false success), absolute `--repo`, `launch --config`.
- VM liveness truth: `ssh.VerifyFirecracker` reads `/proc/<pid>/cmdline`
  (world-readable) instead of `Signal(0)` (EPERM false-negative for root-owned
  processes), so `ps` reports truthful running/stopped without root.
- Data layout: `/var/lib/cell` root-owned 0755, session files world-readable
  (0644), SSH keys 0600 root.

## Gates

1. `./cell ps` → exit 0 without root.
2. `./cell launch --repo .` without root → stderr exactly
   `error: cell: launch requires root — run: sudo cell launch`, exit 1.
3. `go build ./... && go vet ./... && go test ./...` → green.
4. `grep -rn "internal/privilege" --include="*.go" .` → no references.

## Gate evidence

```
$ ./cell ps; echo rc=$?
rc=0
$ ./cell launch --repo . ; echo rc=$?
error: cell: launch requires root — run: sudo cell launch
rc=1
$ go build ./... && go vet ./... && go test ./...   # 0 failures, 11 ok packages
$ grep -rn "internal/privilege" --include="*.go" .   # (no output)
```
