# E2E guest probes over TCP, not ICMP

Date: 2026-08-22

## Problem

`assert_isolation` SSHes as `agent` and runs `ping`. Guest `ping` has no `cap_net_raw` and is not setuid:

```
ping: socket: Operation not permitted
ping: => missing cap_net_raw+p capability or setuid?
```

ICMP never leaves the guest. Guest-to-guest “blocked” is a false pass (same error). Internet check then fails with the same error, mislabeled as “cannot reach internet”.

`ssh_guest` also omits `-F /dev/null`, so host SSSD `KnownHostsCommand` can hang the same way cell used to.

## Goal

- Isolation and egress assertions use TCP as `agent` (curl is already in the guest rootfs).
- Isolation probes a port that is open on B (`:22`). Egress probes `1.1.1.1:443`.
- `ssh_guest` ignores host ssh_config (`-F /dev/null`) and uses `BatchMode=yes`.

## Non-goals

- Giving `ping` capabilities or setuid in the rootfs.
- Changing firewall, TAP isolation, or Go code.
- Installing python/netcat in the guest.
- Running e2e in this change (operator reruns `sudo scripts/e2e-network.sh ./cell`).

## Design

1. `ssh_guest`: add `-F /dev/null`, `-o BatchMode=yes`, `-o ConnectTimeout=5`. Keep `-i`, `IdentitiesOnly`, `StrictHostKeyChecking=no`, `UserKnownHostsFile=/dev/null`.
2. Guest TCP helper (via `ssh_guest`):  
   `curl --connect-only --connect-timeout 2 --max-time 3 -sS -o /dev/null "http://HOST:PORT/"`  
   Exit 0 means TCP connect succeeded. `--connect-only` so sshd on `:22` does not have to speak HTTP.
3. Isolation: A → B `:22` must **fail**. If it succeeds, isolation is broken (`die`).
4. Egress: A → `1.1.1.1:443` must **succeed**.
5. Drop host `ping` from `preflight` required tools. Do not call `ping` in the script.

Amends `docs/superpowers/specs/2026-08-22-e2e-network-smoke-design.md` (ping → TCP). That file is gitignored; this spec is the source of truth for the change.

## Files

- `scripts/e2e-network.sh` — `ssh_guest`, `assert_isolation`, `preflight`

## Success

- `bash -n scripts/e2e-network.sh` exits 0.
- Script contains no `ping` invocation (host `command -v ping` in preflight removed).
- `ssh_guest` includes `-F` `/dev/null` and `BatchMode=yes`.
- Isolation URL uses B’s `guest_ip` port 22; egress uses `1.1.1.1` port 443.
