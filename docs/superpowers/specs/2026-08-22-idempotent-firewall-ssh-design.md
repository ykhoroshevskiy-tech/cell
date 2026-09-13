# Idempotent CELL_* firewall and non-interactive SSH probes

Date: 2026-08-22

## Problem

`sudo scripts/e2e-network.sh ./cell` launches VM A, then fails on VM B:

```
Session 09c408fda80b ready at 172.16.107.2
error: exit status 1
e2e: ERROR: launch failed for …/repo-b
(agent@172.16.107.2) Password
```

A is the first process to install global `CELL_*` iptables chains. B reconciles the same host firewall and dies. The SSH password prompt is a separate automation bug on the same run.

## Goal

- Second and later `launch`/`start`/`stop`/`rm` must reuse existing `CELL_INPUT`, `CELL_FORWARD`, and `CELL_NAT` without failing.
- Failed host net commands must include the command and its output, not a bare `exit status 1`.
- Guest SSH used for health probes and tunnels must never wait for a password on a TTY.

## Non-goals

- Changing firewall policy (ports, MASQUERADE, isolation rules).
- Migrating to nftables-native syntax.
- Rewriting the e2e script beyond what is needed to re-run the existing smoke.
- Interactive password login to guests (keys only).

## Root cause

`EnsureFirewall` always calls `iptables -N` (and `-t nat -N`). When the chain already exists, iptables exits 1 and prints `Chain already exists.` on stderr.

`run()` returns only the `os/exec` error (`exit status 1`) and drops combined output. `ensureChain` looks for `"already exists"` in `err.Error()`, does not find it, and fails the second reconcile.

`sshBaseArgs` does not set `BatchMode=yes`. `ServerReady` (and other SSH helpers that share those args) can prompt `(agent@…) Password` when the key is rejected or the guest is the wrong VM.

## Design

### Firewall chains

Before creating a chain, test existence with `iptables -nL <chain>` (filter) or `iptables -t <table> -nL <chain>` (nat). If that succeeds, skip `-N`.

If `-N` still races and fails, treat an error whose text contains `already exists` as success. Any other `-N` failure remains fatal.

Jump rules (`-I INPUT/FORWARD/POSTROUTING -j CELL_*`), flush, and rule install stay as they are: delete-then-insert jump, flush chain, append the global rule set. That path is already idempotent once the chains exist.

### Command errors

`run()` must wrap failures as:

```
<cmd> <args>: <exec error>
<stdout+stderr>
```

so a future e2e/host failure names the iptables/ip invocation instead of `error: exit status 1`.

### SSH

Add `-o BatchMode=yes` to `sshBaseArgs` so every cell SSH (health probe, tunnel, `cell ssh` key auth) fails closed instead of reading a password from the terminal. Guests are key-only; a password prompt is never a valid recovery path.

Update the tunnel-args unit test to expect `BatchMode=yes`.

## Files

- `internal/network/firewall.go` — existence check in `ensureChain`
- `internal/network/runner.go` — include command output on error
- `internal/ssh/attach.go` — `BatchMode=yes` in `sshBaseArgs`
- Tests: `EnsureFirewall` twice against the fake runner; tunnel SSH args

## Success criteria

- `go test ./...` passes.
- `EnsureFirewall` twice does not fail when chains already exist (unit test with fake runner is required; live iptables is covered by e2e).
- After rebuild: `sudo scripts/e2e-network.sh ./cell` gets past launching VM B (two distinct guest IPs). Remaining smoke steps follow the e2e spec.
- The run does not block on `(agent@…) Password`.

## Out of scope this change

Host reboot → `cell start`. Guest-to-guest isolation and lease reuse remain e2e assertions, not this bugfix’s unit tests.
