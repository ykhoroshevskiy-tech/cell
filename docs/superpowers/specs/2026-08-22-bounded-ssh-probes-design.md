# Bounded SSH probes in wait-ready

Date: 2026-08-22

## Problem

`cell -v launch` after `InstanceStart` prints one line:

```
ssh: waiting for 172.16.107.2:22 (0s remaining)
```

then goes silent for a long time. Operators treat that as a hang. `0s remaining` is the inner `WaitForSSH(..., 1s)` used by `SessionStatus`, not the 90s ready timeout.

After a later poll sees TCP/22 open (or a SYN that OpenSSH accepts), `ServerReady` runs `ssh agent@guest curl …/health` with **no `ConnectTimeout`**. OpenSSH can block minutes with no output. `BatchMode=yes` only disables the password prompt.

## Goal

- Each wait-ready iteration finishes in well under a second when SSH is down, and in a few seconds when probing health.
- Health/tunnel SSH cannot stall on TCP.
- Verbose wait still prints `wait: vm= ssh= runtime= server=` every loop.
- Serial dump on `-v` stays as it is today.

## Non-goals

- Skipping OpenCode health in `launch`.
- Changing firewall or bridge behavior.
- Rewriting the e2e script in this change (memory/vcpu already adjusted separately).

## Design

1. Add `-o ConnectTimeout=5` to `sshBaseArgs` (shared by `ServerReady`, tunnel, `cell ssh`).
2. `SessionStatus` must not call `WaitForSSH` (that function waits and logs). Use one `net.DialTimeout` (~200ms) for `SSHReachable`.
3. Keep `WaitForSSH` for `cell pull` and any explicit wait.
4. Do not remove `WaitRuntimeReady` serial-to-stderr when verbose.

## Files

- `internal/ssh/attach.go`
- `internal/ssh/attach_test.go` (`TestTunnelSSHArgs` expects `ConnectTimeout=5`)

## Success

- After rebuild, `-v launch` shows repeating `wait: vm=… ssh=… server=… (Ns remaining)` while the guest boots, not a single SSH line then silence.
- `go test ./internal/ssh` passes.
- A black-hole guest IP does not block `ServerReady` longer than ~5s per probe.
