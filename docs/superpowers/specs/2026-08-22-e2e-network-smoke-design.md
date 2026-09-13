# E2E network smoke script

Date: 2026-08-22

## Problem

Bridge network lifecycle is covered by unit tests with a fake command runner. Nothing exercises the built `cell` binary against real `cell0`, TAPs, iptables, and two live Firecracker VMs.

## Goal

After `go build`, run `sudo scripts/e2e-network.sh ./cell` to smoke-test:

- Two concurrent VMs with distinct guest IPs on `172.16.107.0/24`
- Host bridge/TAP layout (no TAP IPs, port isolation, single subnet route)
- Guest-to-guest isolation (ping fails) and internet egress (ping succeeds)
- Stale/orphan TAP cleanup on reconcile
- IP lease reuse after `stop` + `rm`

## Non-goals

- TUI / `cell attach`
- OpenCode-specific health beyond launch's existing wait-ready
- Concurrent `launch` races
- Host reboot → `cell start` (manual README note)
- CI wiring or Makefile

## Architecture

Black-box shell script. Isolated session data via `CELL_DATA_DIR` temp dir; reuse bootstrapped kernel/rootfs/firecracker from `/var/lib/cell/images` via explicit `CELL_*_PATH` overrides (setting only `CELL_DATA_DIR` would relocate images under the temp dir).

Reconcile is triggered by `cell stop` / `cell rm`, not `cell status` / `cell ps`.

Guest checks use direct SSH (`ssh -i …/id_ed25519 agent@<ip>`), not interactive `cell ssh`.

Cleanup always stops/rms e2e sessions. Tear down `cell0` and `CELL_*` iptables chains only when `/var/lib/cell` has no running VMs and no `ctap-*` interfaces remain.

## Success criteria

Script exits 0 after all assertions; exits 1 on first failure with a clear message. Skips nothing silently when root/KVM/artifacts are missing.
